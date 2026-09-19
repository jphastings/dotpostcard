package atproto

import (
	"encoding/json"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testBlob = Blob{Type: "blob", Ref: BlobRef{Link: "bafyTestCID"}, MimeType: "image/jpeg", Size: 1234}

func TestFromMetadataToMetadataRoundTrip(t *testing.T) {
	meta := testhelpers.SamplePostcard.Meta

	record := FromMetadata(meta, testBlob)
	got, err := record.ToMetadata()
	require.NoError(t, err)

	assert.Equal(t, meta, got)
	assert.Equal(t, testBlob, record.Image)
}

func TestRecordJSONShape(t *testing.T) {
	lat := 45.28
	meta := types.Metadata{
		Flip:     types.FlipBook,
		Location: types.Location{Latitude: &lat},
	}

	data, err := json.Marshal(FromMetadata(meta, testBlob))
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Equal(t, RecordType, raw["$type"])
	assert.Equal(t, "org.dotpostcard.postcard#flipBook", raw["flip"])

	image, ok := raw["image"].(map[string]any)
	require.True(t, ok, "image should be an object")
	assert.Equal(t, "blob", image["$type"])
	assert.Equal(t, testBlob.Ref.Link, image["ref"].(map[string]any)["$link"])
	assert.Equal(t, testBlob.MimeType, image["mimeType"])

	location, ok := raw["location"].(map[string]any)
	require.True(t, ok, "location should be an object")
	assert.IsType(t, "", location["latitude"], "latitude must be encoded as a string, not a number")
	assert.Equal(t, "45.28", location["latitude"])
}

func TestDiffReportsNothingForIdenticalRecords(t *testing.T) {
	meta := testhelpers.SamplePostcard.Meta
	a := FromMetadata(meta, testBlob)
	b := FromMetadata(meta, testBlob)

	assert.Empty(t, Diff(a, b))
}

func TestDiffReportsEditedTranscription(t *testing.T) {
	meta := testhelpers.SamplePostcard.Meta
	a := FromMetadata(meta, testBlob)
	b := FromMetadata(meta, testBlob)
	b.Sides[1].Transcription.Text = "Something else entirely"

	assert.Equal(t, []string{"sides[1].transcription"}, Diff(a, b))
}

func secretMeta(prehidden bool) types.Metadata {
	return types.Metadata{
		Flip: types.FlipNone,
		Front: types.Side{
			Secrets: []types.Polygon{{
				Prehidden: prehidden,
				Points:    []types.Point{{X: 0.1, Y: 0.1}, {X: 0.2, Y: 0.1}, {X: 0.2, Y: 0.2}},
			}},
		},
	}
}

func marshalledSecret(t *testing.T, record Record) map[string]any {
	t.Helper()

	data, err := json.Marshal(record)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	sides := raw["sides"].([]any)
	secrets := sides[0].(map[string]any)["secrets"].([]any)
	return secrets[0].(map[string]any)
}

func TestSecretPrehiddenTrueOmitsFieldFromJSON(t *testing.T) {
	record := FromMetadata(secretMeta(true), testBlob)
	assert.NotContains(t, marshalledSecret(t, record), "prehidden")
}

func TestSecretPrehiddenFalseRoundTrips(t *testing.T) {
	record := FromMetadata(secretMeta(false), testBlob)
	assert.Equal(t, false, marshalledSecret(t, record)["prehidden"])

	got, err := record.ToMetadata()
	require.NoError(t, err)
	assert.False(t, got.Front.Secrets[0].Prehidden)
}

func TestSecretPrehiddenMissingFromJSONReadsAsTrue(t *testing.T) {
	raw := `{
		"$type": "org.dotpostcard.postcard",
		"image": {"$type": "blob", "ref": {"$link": "x"}, "mimeType": "image/jpeg", "size": 1},
		"flip": "org.dotpostcard.postcard#flipNone",
		"sides": [{"secrets": [{"points": [{"x":1000,"y":1000},{"x":2000,"y":1000},{"x":2000,"y":2000}]}]}],
		"frontSize": {"widthPx": 1, "heightPx": 1}
	}`

	var record Record
	require.NoError(t, json.Unmarshal([]byte(raw), &record))

	meta, err := record.ToMetadata()
	require.NoError(t, err)
	assert.True(t, meta.Front.Secrets[0].Prehidden)
}

func TestValidRecordKey(t *testing.T) {
	assert.NoError(t, ValidRecordKey("san-marino_1974"))
	assert.Error(t, ValidRecordKey("has space"))
	assert.Error(t, ValidRecordKey("."))
	assert.Error(t, ValidRecordKey(".."))
	assert.Error(t, ValidRecordKey(""))
}
