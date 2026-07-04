package collection

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

func TestCreateOpenLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.postcard.db")

	col, err := Create(path)
	assert.NoError(t, err)
	assert.NoError(t, col.Close())

	_, err = Create(path)
	assert.Error(t, err, "Create should error when the file already exists")

	rw, err := Open(path)
	assert.NoError(t, err)
	assert.NoError(t, rw.Close())

	ro, err := OpenReadOnly(path)
	assert.NoError(t, err)
	assert.NoError(t, ro.Close())
}

func TestOpenRejectsNonCollectionFiles(t *testing.T) {
	garbage := filepath.Join(t.TempDir(), "garbage.postcard.db")
	assert.NoError(t, os.WriteFile(garbage, []byte("not a sqlite database"), 0644))

	_, err := Open(garbage)
	assert.Error(t, err)

	_, err = OpenReadOnly(garbage)
	assert.Error(t, err)
}

func TestAddListMetadataRoundTrip(t *testing.T) {
	data, filename, wantMeta, wantFront := encodeSample(t)
	col := mustCreate(t)

	summary, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	assert.Equal(t, "some-postcard", summary.Name)
	assert.Equal(t, filename, summary.Filename)
	assert.Equal(t, "image/jpeg", summary.Mimetype)
	assert.Equal(t, wantMeta.Flip, summary.Flip)
	assert.True(t, summary.HasBack)
	assert.Equal(t, wantMeta.SentOn, summary.SentOn)
	assert.Equal(t, wantMeta.Sender.Name, summary.SenderName)
	assert.Equal(t, wantMeta.Recipient.Name, summary.RecipientName)
	assert.Equal(t, wantMeta.Location.Name, summary.LocationName)
	assert.Equal(t, wantMeta.Location.CountryCode, summary.CountryCode)
	assert.Equal(t, wantMeta.Location.Latitude, summary.Latitude)
	assert.Equal(t, wantMeta.Location.Longitude, summary.Longitude)
	assert.Equal(t, wantFront.Bounds().Dx(), summary.FrontPxW)
	assert.Equal(t, wantFront.Bounds().Dy(), summary.FrontPxH)

	list, err := col.List()
	assert.NoError(t, err)
	assert.Equal(t, []CardSummary{summary}, list)

	meta, err := col.Metadata(summary.Name)
	assert.NoError(t, err)
	assert.Equal(t, wantMeta, meta)

	_, err = col.Metadata("no-such-card")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListOrdering(t *testing.T) {
	col := mustCreate(t)

	date := func(y int, m time.Month, d int) *types.Date {
		return &types.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
	}

	cards := []struct {
		name   string
		sentOn *types.Date
	}{
		{"older", date(2001, time.January, 1)},
		{"newest", date(2010, time.June, 15)},
		{"tie-b", date(2005, time.March, 3)},
		{"tie-a", date(2005, time.March, 3)},
		{"undated-b", nil},
		{"undated-a", nil},
	}

	for _, c := range cards {
		data, filename := encodeNamed(t, c.name, c.sentOn)
		_, err := col.AddWebPostcard(filename, data)
		assert.NoError(t, err)
	}

	list, err := col.List()
	assert.NoError(t, err)

	names := make([]string, len(list))
	for i, s := range list {
		names[i] = s.Name
	}

	assert.Equal(t, []string{"newest", "tie-a", "tie-b", "older", "undated-a", "undated-b"}, names)
}

func TestCardData(t *testing.T) {
	data, filename, _, _ := encodeSample(t)
	col := mustCreate(t)

	summary, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	gotData, mimetype, err := col.CardData(summary.Name)
	assert.NoError(t, err)
	assert.Equal(t, data, gotData)
	assert.Equal(t, "image/jpeg", mimetype)

	_, _, err = col.CardData("no-such-card")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAddWebPostcardIsIdempotent(t *testing.T) {
	data, filename, _, _ := encodeSample(t)
	col := mustCreate(t)

	first, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	second, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)
	assert.Equal(t, first, second)

	list, err := col.List()
	assert.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestRemove(t *testing.T) {
	data, filename, _, _ := encodeSample(t)
	col := mustCreate(t)

	summary, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	assert.NoError(t, col.Remove(summary.Name))

	list, err := col.List()
	assert.NoError(t, err)
	assert.Empty(t, list)

	_, _, err = col.CardData(summary.Name)
	assert.ErrorIs(t, err, ErrNotFound)

	results, err := col.Search(summary.SenderName)
	assert.NoError(t, err)
	assert.Empty(t, results, "removed card should no longer be found by search")

	assert.ErrorIs(t, col.Remove(summary.Name), ErrNotFound)
}

func TestOpenReadOnly(t *testing.T) {
	data, filename, _, _ := encodeSample(t)
	path := filepath.Join(t.TempDir(), "test.postcard.db")

	col, err := Create(path)
	assert.NoError(t, err)
	_, err = col.AddWebPostcard(filename, data)
	assert.NoError(t, err)
	assert.NoError(t, col.Close())

	ro, err := OpenReadOnly(path)
	assert.NoError(t, err)
	defer ro.Close()

	list, err := ro.List()
	assert.NoError(t, err)
	assert.Len(t, list, 1)

	results, err := ro.Search(list[0].SenderName)
	assert.NoError(t, err)
	assert.NotEmpty(t, results)

	_, err = ro.AddWebPostcard(filename, data)
	assert.ErrorIs(t, err, errReadOnly)

	assert.ErrorIs(t, ro.Remove(list[0].Name), errReadOnly)
}
