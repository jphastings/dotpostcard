package appcore

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompilePostcardRoundTripsIntoCollection(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")
	back := testhelpers.RawTestImage("sample-back.png")
	metaJSON := `{"sender":{"name":"Alice"},"recipient":{"name":"Bob"},"flip":"book","front":{"description":"Front desc"},"back":{"description":"Back desc"}}`

	cc, err := CompilePostcard("card-one", metaJSON, front, back, false, false)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(cc.Filename(), "card-one.postcard."), "filename %q must be {name}.postcard.{ext}", cc.Filename())
	assert.NotEmpty(t, cc.Mimetype())
	assert.NotEmpty(t, cc.Data())

	path := buildCollection(t)
	summaryJSON, err := AddCardToCollection(path, cc.Filename(), cc.Data())
	require.NoError(t, err)

	var summary collection.CardSummary
	require.NoError(t, json.Unmarshal([]byte(summaryJSON), &summary))
	assert.Equal(t, "card-one", summary.Name)
	assert.Equal(t, "Alice", summary.SenderName)
	assert.Equal(t, "Bob", summary.RecipientName)
}

func TestCompilePostcardFlipValidation(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")
	back := testhelpers.RawTestImage("sample-back.png")

	tests := []struct {
		name       string
		back       []byte
		metaJSON   string
		wantErrSub string
	}{
		{
			name:     "front-only with flip none succeeds",
			back:     nil,
			metaJSON: `{"flip":"none"}`,
		},
		{
			// Two sides are present, but "none" isn't a valid flip for a
			// two-sided card: this is the Validate() error a two-sided
			// compile can actually hit (a front-only compile always
			// normalizes to flip "none" itself, regardless of what's asked).
			name:       "two-sided with flip none errors",
			back:       back,
			metaJSON:   `{"flip":"none"}`,
			wantErrSub: "flip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CompilePostcard("flip-check", tt.metaJSON, front, tt.back, false, false)
			if tt.wantErrSub == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErrSub)
			}
		})
	}
}

func TestCompilePostcardForcedPhysicalSizeSurvivesRoundTrip(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")
	metaJSON := `{"flip":"none","physical":{"frontSize":{"cmW":"14.8","cmH":"10.5"}}}`

	cc, err := CompilePostcard("forced-size", metaJSON, front, nil, false, false)
	require.NoError(t, err)

	path := writeBareFile(t, cc.Filename(), cc.Data())
	cf, err := OpenCardFile(path)
	require.NoError(t, err)

	metaJSONOut, err := cf.MetaJSON()
	require.NoError(t, err)

	var probe struct {
		Physical struct {
			FrontDimensions types.Size `json:"frontSize"`
		} `json:"physical"`
	}
	require.NoError(t, json.Unmarshal([]byte(metaJSONOut), &probe))
	require.True(t, probe.Physical.FrontDimensions.HasPhysical())
	assert.Equal(t, "14.8000", probe.Physical.FrontDimensions.CmWidth.FloatString(4))
	assert.Equal(t, "10.5000", probe.Physical.FrontDimensions.CmHeight.FloatString(4))
}

func TestCompilePostcardOrientationMismatchErrors(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png") // landscape
	back := testhelpers.RawTestImage("sample-portrait-back.png")

	_, err := CompilePostcard("mismatch", `{"flip":"book"}`, front, back, false, false)
	assert.ErrorContains(t, err, "orientation")
}

// TestCompilePostcardBoxSecretSurvivesRoundTrip compiles a card with an
// unhidden box secret and confirms its geometry rides through to the
// compiled file: the box (left/top/width/height) is converted to a
// clockwise quad of corner points on the way in (types.SecretBox.intoPolygon),
// and hideSecrets marks it Prehidden once its pixels are painted over. This
// asserts against the raw "points"/"prehidden" JSON — rather than
// unmarshalling into types.Polygon, which MetaJSON's output now supports,
// see cardfile.go's jsonSafeMetadata doc comment — to keep this test focused
// on secret geometry rather than JSON shape.
func TestCompilePostcardBoxSecretSurvivesRoundTrip(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")
	metaJSON := `{"flip":"none","front":{"secrets":[{"type":"box","prehidden":false,"left":0.1,"top":0.2,"width":0.3,"height":0.2}]}}`

	cc, err := CompilePostcard("secret-card", metaJSON, front, nil, false, false)
	require.NoError(t, err)

	path := writeBareFile(t, cc.Filename(), cc.Data())
	cf, err := OpenCardFile(path)
	require.NoError(t, err)

	metaJSONOut, err := cf.MetaJSON()
	require.NoError(t, err)

	var probe struct {
		Front struct {
			Secrets []struct {
				Prehidden bool          `json:"prehidden"`
				Points    []types.Point `json:"points"`
			} `json:"secrets"`
		} `json:"front"`
	}
	require.NoError(t, json.Unmarshal([]byte(metaJSONOut), &probe))
	require.Len(t, probe.Front.Secrets, 1)
	assert.True(t, probe.Front.Secrets[0].Prehidden, "secret pixels are painted over during compile, so Prehidden must be true by the time it's stored")
	assert.Equal(t, []types.Point{
		{X: 0.1, Y: 0.2},
		{X: 0.4, Y: 0.2},
		{X: 0.4, Y: 0.4},
		{X: 0.1, Y: 0.4},
	}, probe.Front.Secrets[0].Points)
}

func TestCompilePostcardBoxSecretOutOfBoundsErrors(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")
	metaJSON := `{"flip":"none","front":{"secrets":[{"type":"box","prehidden":false,"left":0.9,"top":0.2,"width":0.3,"height":0.2}]}}`

	_, err := CompilePostcard("oob-secret", metaJSON, front, nil, false, false)
	assert.ErrorContains(t, err, "edge")
}

func TestCompilePostcardRemoveBorderProducesTransparency(t *testing.T) {
	scan := testhelpers.RawTestImage("removeborder-seattle-scan.jpeg")

	cc, err := CompilePostcard("removeborder-card", `{"flip":"none"}`, scan, nil, true, false)
	require.NoError(t, err)

	assert.NotEqual(t, "image/jpeg", cc.Mimetype(), "a border-removed card needs transparency, which jpeg can't encode")
	assert.False(t, strings.HasSuffix(cc.Filename(), ".jpeg") || strings.HasSuffix(cc.Filename(), ".jpg"))
}

func TestCompilePostcardArchivalProducesLosslessOutput(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")
	back := testhelpers.RawTestImage("sample-back.png")

	cc, err := CompilePostcard("archival-card", `{"flip":"book"}`, front, back, false, true)
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(cc.Filename(), ".webp"), "archival output must be lossless, and jpeg can't encode losslessly: got %q", cc.Filename())
}

func TestCompilePostcardGarbageInputsErrorWithoutPanicking(t *testing.T) {
	front := testhelpers.RawTestImage("sample-front.png")

	_, err := CompilePostcard("bad-meta", "not json", front, nil, false, false)
	assert.Error(t, err)

	_, err = CompilePostcard("bad-front", `{}`, []byte("not an image"), nil, false, false)
	assert.Error(t, err)
}
