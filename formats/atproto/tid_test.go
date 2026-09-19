package atproto

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tidRE = regexp.MustCompile(`^[234567abcdefghij][234567abcdefghijklmnopqrstuvwxyz]{12}$`)

// tidTimestamp decodes a TID's 53-bit microsecond field back to a date, so tests can check
// RecordKey put the right day in it.
func tidTimestamp(t *testing.T, tid string) time.Time {
	t.Helper()
	require.Regexp(t, tidRE, tid)

	var v uint64
	for i := 0; i < len(tid); i++ {
		idx := strings.IndexByte(tidAlphabet, tid[i])
		require.GreaterOrEqualf(t, idx, 0, "character %q isn't in the TID alphabet", tid[i])
		v = v<<5 | uint64(idx)
	}

	return time.UnixMicro(int64(v >> 10)).UTC()
}

func TestRecordKeyMatchesTIDFormat(t *testing.T) {
	key := RecordKey(nil, time.Now(), []byte("some image bytes"))
	assert.Regexp(t, tidRE, key)
}

func TestRecordKeyIsDeterministic(t *testing.T) {
	sentOn := &types.Date{Time: time.Date(1974, time.September, 26, 0, 0, 0, 0, time.UTC)}
	today := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	image := []byte("the same image bytes")

	assert.Equal(t, RecordKey(sentOn, today, image), RecordKey(sentOn, today, image))
}

func TestRecordKeyVariesWithImage(t *testing.T) {
	sentOn := &types.Date{Time: time.Date(1974, time.September, 26, 0, 0, 0, 0, time.UTC)}
	today := time.Now()

	a := RecordKey(sentOn, today, []byte("image one"))
	b := RecordKey(sentOn, today, []byte("image two"))
	assert.NotEqual(t, a, b)
}

func TestRecordKeyTimestampMatchesSentOnDate(t *testing.T) {
	sentOn := &types.Date{Time: time.Date(1974, time.September, 26, 0, 0, 0, 0, time.UTC)}
	key := RecordKey(sentOn, time.Now(), []byte("some image bytes"))

	assert.Equal(t, "1974-09-26", tidTimestamp(t, key).Format("2006-01-02"))
}

func TestRecordKeyClampsPre1970SentOnToEpoch(t *testing.T) {
	sentOn := &types.Date{Time: time.Date(1905, time.March, 3, 0, 0, 0, 0, time.UTC)}
	key := RecordKey(sentOn, time.Now(), []byte("some image bytes"))

	assert.Equal(t, "1970-01-01", tidTimestamp(t, key).Format("2006-01-02"))
}

func TestRecordKeyUsesTodayWhenSentOnUnknown(t *testing.T) {
	today := time.Date(2020, time.June, 15, 0, 0, 0, 0, time.UTC)
	key := RecordKey(nil, today, []byte("some image bytes"))

	assert.Equal(t, "2020-06-15", tidTimestamp(t, key).Format("2006-01-02"))
}
