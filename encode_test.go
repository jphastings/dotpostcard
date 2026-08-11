package postcards

import (
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

// TestOutputNamesPredictEncodedFilenames is a drift guard: it stops Encode and
// OutputNames silently diverging as codecs change. For every registered codec and every
// flag combination that can change what web predicts, it encodes a fixture postcard and
// asserts every (non-shared) filename Encode actually produced was among the names
// OutputNames predicted for it beforehand. Not every codec supports every combination
// (eg. svg can't be archival) — when Encode itself errors, there's nothing to check.
func TestOutputNamesPredictEncodedFilenames(t *testing.T) {
	pc := testhelpers.SamplePostcard
	// testhelpers.SamplePostcard.Front/Back are nil: fixtures.go builds that struct
	// literal from testhelpers.TestImages before TestImages is populated (that happens
	// in an init() func, which Go runs after package-level var initializers).
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	assert.NotNil(t, pc.Front)
	assert.NotNil(t, pc.Back)

	variants := map[string]formats.EncodeOptions{
		"default":         {},
		"archival":        {Archival: true},
		"no-transparency": {NoTransparency: true},
	}

	for _, codecName := range Codecs {
		codec := codecs[codecName]

		for variantName, variantOpts := range variants {
			t.Run(codecName+"/"+variantName, func(t *testing.T) {
				opts := variantOpts // a codec's Encode must not be able to affect other cases' opts

				fws, err := codec.Encode(pc, &opts)
				if err != nil {
					return
				}

				predicted := codec.OutputNames(pc.Name, &variantOpts)
				for _, fw := range fws {
					if fw.Shared {
						continue
					}
					assert.Contains(t, predicted, fw.Filename,
						"%s/%s: Encode produced %q but OutputNames didn't predict it", codecName, variantName, fw.Filename)
				}
			})
		}
	}
}
