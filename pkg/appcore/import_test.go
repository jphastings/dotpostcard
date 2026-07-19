package appcore

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// importProbe decodes just enough of a canonical metadata JSON blob to
// assert on secrets' "type" discriminator directly, since types.Polygon
// can't be unmarshalled into without it (see jsonSafeMetadata's doc comment
// in cardfile.go).
type importProbe struct {
	Sender    types.Person `json:"sender"`
	Recipient types.Person `json:"recipient"`
	Location  types.Location
	Flip      types.Flip
	SentOn    *types.Date `json:"sentOn"`
	Front     struct {
		Secrets []struct {
			Type      string `json:"type"`
			Prehidden bool   `json:"prehidden"`
		} `json:"secrets"`
	} `json:"front"`
	Physical struct {
		FrontSize struct {
			CmW string `json:"cmW"`
			CmH string `json:"cmH"`
		} `json:"frontSize"`
	} `json:"physical"`
}

func TestMetaJSONFromCardBytesMatchesOpenCardFile(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")
	back := testhelpers.RawTestImage("sample-back.png")
	metaJSON := `{"flip":"book","sender":{"name":"Alice"},"recipient":{"name":"Bob"},` +
		`"front":{"secrets":[{"type":"box","prehidden":false,"left":0.1,"top":0.2,"width":0.3,"height":0.2}]},` +
		`"physical":{"frontSize":{"cmW":"14.8","cmH":"10.5"}}}`

	cc, err := CompilePostcard("import-card", metaJSON, front, back, false, false)
	require.NoError(t, err)

	gotJSON, err := MetaJSONFromCardBytes(cc.Filename(), cc.Data())
	require.NoError(t, err)

	path := writeBareFile(t, cc.Filename(), cc.Data())
	cf, err := OpenCardFile(path)
	require.NoError(t, err)
	wantJSON, err := cf.MetaJSON()
	require.NoError(t, err)

	assert.JSONEq(t, wantJSON, gotJSON)

	var probe importProbe
	require.NoError(t, json.Unmarshal([]byte(gotJSON), &probe))
	assert.Equal(t, "Alice", probe.Sender.Name)
	assert.Equal(t, "Bob", probe.Recipient.Name)
	assert.Equal(t, types.FlipBook, probe.Flip)
	require.Len(t, probe.Front.Secrets, 1)
	assert.Equal(t, "polygon", probe.Front.Secrets[0].Type, `a compiled secret must re-marshal with its "type" discriminator so it can be fed back into CompilePostcard`)
	assert.True(t, probe.Front.Secrets[0].Prehidden, "secret pixels are painted over during compile")
	assert.Equal(t, "14.8000", parseRatString(t, probe.Physical.FrontSize.CmW))
	assert.Equal(t, "10.5000", parseRatString(t, probe.Physical.FrontSize.CmH))

	// The JSON this produces must itself be a valid CompilePostcard input.
	_, err = CompilePostcard("import-card-roundtrip", gotJSON, front, back, false, false)
	assert.NoError(t, err)
}

func TestMetaJSONFromCardBytesGarbageInputsError(t *testing.T) {
	_, err := MetaJSONFromCardBytes("not-an-image.jpg", []byte("not an image"))
	assert.Error(t, err)

	pngNoXMP := testhelpers.RawTestImage("sample-front.png")
	_, err = MetaJSONFromCardBytes("sample-front.png", pngNoXMP)
	assert.Error(t, err)
}

func TestMetaJSONFromComponentYAML(t *testing.T) {
	gotJSON, err := MetaJSONFromComponentYAML(testhelpers.SampleYAML)
	require.NoError(t, err)

	var probe importProbe
	require.NoError(t, json.Unmarshal([]byte(gotJSON), &probe))

	assert.Equal(t, "Front, Italy", probe.Location.Name)
	assert.Equal(t, "ITA", probe.Location.CountryCode)
	assert.Equal(t, types.FlipBook, probe.Flip)
	require.NotNil(t, probe.SentOn)
	assert.Equal(t, "2006-01-02", probe.SentOn.Time.Format("2006-01-02"))
	assert.Equal(t, "Alice", probe.Sender.Name)
	assert.Equal(t, "Bob", probe.Recipient.Name)
	require.Len(t, probe.Front.Secrets, 1)
	assert.Equal(t, "polygon", probe.Front.Secrets[0].Type, `sample-meta.yaml's secrets must re-marshal with their "type" discriminator`)
	assert.False(t, probe.Front.Secrets[0].Prehidden)
	assert.Equal(t, "14.8000", parseRatString(t, probe.Physical.FrontSize.CmW))
	assert.Equal(t, "10.5000", parseRatString(t, probe.Physical.FrontSize.CmH))

	// A front-only compile always normalizes flip to "none" itself
	// (formats/component's Decode does this whenever no back file is
	// given), so sample-meta.yaml's "book" flip doesn't need adjusting
	// here for the round trip to succeed.
	front := testhelpers.RawTestImage("sample-front.png")
	_, err = CompilePostcard("yaml-import-card", gotJSON, front, nil, false, false)
	assert.NoError(t, err)
}

func TestMetaJSONFromComponentYAMLGarbageInputErrors(t *testing.T) {
	_, err := MetaJSONFromComponentYAML([]byte("not: valid: yaml: at: all: [1,2"))
	assert.Error(t, err)
}

func TestComponentYAMLFromMetaJSONRoundTripsThroughMetaJSONFromComponentYAML(t *testing.T) {
	metaJSON := `{"locale":"en-GB","location":{"name":"Front, Italy","latitude":45.28,"longitude":7.66,"countrycode":"ITA"},` +
		`"flip":"book","sentOn":"2006-01-02",` +
		`"sender":{"name":"Alice","uri":"https://alice.example.com"},` +
		`"recipient":{"name":"Bob","uri":"https://bob.example.org"},` +
		`"front":{"description":"The word 'Front' in large blue letters","transcription":{"text":"Front"},` +
		`"secrets":[{"type":"box","prehidden":true,"left":0.3,"top":0.6,"width":0.1,"height":0.2}]},` +
		`"back":{"description":"The word 'Back' in large red letters",` +
		`"transcription":{"text":"Back","annotations":[{"type":"locale","value":"en-GB","start":0,"end":4}]}},` +
		`"context":{"author":{"name":"Carol","uri":"https://carol.example.net"},"description":"This is a sample postcard, with all fields expressed."},` +
		// cmW/cmH are given as rationals here (74/5 == 14.8, 21/2 == 10.5) to
		// exercise the format change through the round trip: YAML always
		// encodes a decimal-ish "14.80cm x 10.50cm" (types.Size.MarshalYAML),
		// so the value coming back out is a different *big.Rat representation
		// of the same number.
		`"physical":{"frontSize":{"cmW":"74/5","cmH":"21/2"},"thicknessMM":0.4,"cardColor":"#E6E6D9"}}`

	yamlOut, err := ComponentYAMLFromMetaJSON(metaJSON)
	require.NoError(t, err)

	for _, want := range []string{"front_size:", "sent_on:", "type: polygon", "card_color:"} {
		assert.Contains(t, string(yamlOut), want)
	}

	roundTripJSON, err := MetaJSONFromComponentYAML(yamlOut)
	require.NoError(t, err)

	var want, got types.Metadata
	require.NoError(t, json.Unmarshal([]byte(metaJSON), &want))
	require.NoError(t, json.Unmarshal([]byte(roundTripJSON), &got))

	assert.Equal(t, want.Locale, got.Locale)
	assert.Equal(t, want.Location, got.Location)
	assert.Equal(t, want.Flip, got.Flip)
	assert.Equal(t, want.SentOn.Time.Format("2006-01-02"), got.SentOn.Time.Format("2006-01-02"))
	assert.Equal(t, want.Sender, got.Sender)
	assert.Equal(t, want.Recipient, got.Recipient)
	assert.Equal(t, want.Front.Description, got.Front.Description)
	assert.Equal(t, want.Front.Transcription, got.Front.Transcription)
	assert.Equal(t, want.Back.Description, got.Back.Description)
	assert.Equal(t, want.Back.Transcription, got.Back.Transcription)
	assert.Equal(t, want.Context, got.Context)
	require.Len(t, got.Front.Secrets, 1)
	// The box secret becomes a polygon on the way in (types.SecretBox.intoPolygon);
	// ComponentYAMLFromMetaJSON's output re-marshals it as "type: polygon", so it
	// comes back out as the equivalent quad rather than the original box shape.
	assert.Equal(t, want.Front.Secrets[0].Points, got.Front.Secrets[0].Points)
	assert.Equal(t, want.Front.Secrets[0].Prehidden, got.Front.Secrets[0].Prehidden)
	// cmW/cmH may round-trip as a differently-formatted rational (eg. "74/5"
	// in vs. a decimal-ish "14.8" out), so compare parsed float values rather
	// than the raw *big.Rat representations.
	wantW, _ := want.Physical.FrontDimensions.CmWidth.Float64()
	gotW, _ := got.Physical.FrontDimensions.CmWidth.Float64()
	assert.InDelta(t, wantW, gotW, 0.001)
	wantH, _ := want.Physical.FrontDimensions.CmHeight.Float64()
	gotH, _ := got.Physical.FrontDimensions.CmHeight.Float64()
	assert.InDelta(t, wantH, gotH, 0.001)
	assert.Equal(t, want.Physical.ThicknessMM, got.Physical.ThicknessMM)
	assert.Equal(t, want.Physical.CardColor, got.Physical.CardColor)
}

func TestComponentYAMLFromMetaJSONMinimalInputProducesMinimalYAML(t *testing.T) {
	yamlOut, err := ComponentYAMLFromMetaJSON(`{"front":{"description":"Just a front description"}}`)
	require.NoError(t, err)

	assert.Equal(t, "front:\n    description: Just a front description\n", string(yamlOut))
}

func TestComponentYAMLFromMetaJSONGarbageInputErrors(t *testing.T) {
	_, err := ComponentYAMLFromMetaJSON("not json{{{")
	assert.Error(t, err)
}

// parseRatString parses a JSON-marshalled *big.Rat string (as produced by
// types.Size.CmWidth/CmHeight's default encoding/json handling) into a
// stable, human-readable form for comparison, mirroring
// TestCompilePostcardForcedPhysicalSizeSurvivesRoundTrip's use of FloatString.
func parseRatString(t *testing.T, s string) string {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	require.True(t, ok, "couldn't parse %q as a big.Rat", s)
	return r.FloatString(4)
}
