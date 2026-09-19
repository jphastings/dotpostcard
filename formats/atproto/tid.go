package atproto

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/jphastings/dotpostcard/types"
)

// tidAlphabet is atproto's base32-sortable encoding.
const tidAlphabet = "234567abcdefghijklmnopqrstuvwxyz"

// tidEpoch is the earliest day a TID can represent: its 53-bit timestamp field can't hold a
// negative microsecond count.
var tidEpoch = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)

// RecordKey derives a TID (atproto's standard record key: a 64-bit value of a zero top bit, a
// 53-bit microsecond timestamp and a 10-bit clock id, base32-encoded to 13 characters) for a
// postcard.
//
// Its timestamp is the day the postcard was sent, or today's if that's unknown — clamped
// forward to 1970-01-01 for postcards sent before the Unix epoch, which is common for
// postcards. Its clock-id, and the time of day within that timestamp, come from the image's
// own hash, so re-uploading an unchanged image reproduces the same key.
func RecordKey(sentOn *types.Date, today time.Time, image []byte) string {
	day := dayUTC(today)
	if sentOn != nil && !sentOn.IsZero() {
		day = dayUTC(sentOn.Time)
	}
	if day.Before(tidEpoch) {
		day = tidEpoch
	}

	// The same digest atproto embeds in the blob's CID: this genuinely is the image's hash,
	// not a separate identifier for it.
	sum := sha256.Sum256(image)
	h := binary.BigEndian.Uint64(sum[:8])

	micros := day.UnixMicro() + int64((h>>10)%86_400_000_000)
	clockID := h & 0x3ff

	return encodeTID(uint64(micros)<<10 | clockID)
}

func dayUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func encodeTID(v uint64) string {
	var out [13]byte
	for i := 12; i >= 0; i-- {
		out[i] = tidAlphabet[v&0x1f]
		v >>= 5
	}
	return string(out[:])
}
