package xmp

import (
	"bytes"
	"io"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/internal/version"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

func TestBundle(t *testing.T) {
	xmpFilenames := []string{"sample.xmp"}
	anyOldFilenames := []string{"any.jpg", "other.json", "files-meta.yaml"}
	group := testhelpers.TestFiles(append(anyOldFilenames, xmpFilenames...))

	bnd, remaining, errs := Codec().Bundle(group)
	assert.Empty(t, errs)

	assert.Len(t, bnd, 1)

	b, ok := bnd[0].(bundle)
	assert.True(t, ok, "unexpected bundle type")

	data, err := io.ReadAll(b.r)
	assert.NoError(t, err)
	assert.Equal(t, string(testhelpers.DataForTestFile("sample.xmp")), string(data))

	assert.Equal(t, anyOldFilenames, testhelpers.Filenames(remaining))
}

func TestEncode(t *testing.T) {
	fws, err := Codec().Encode(testhelpers.SamplePostcard, nil)
	assert.NoError(t, err)

	assert.Len(t, fws, 1)

	content, err := fws[0].Bytes()
	assert.NoError(t, err)

	// Possibly not set to v0.0.0 here because this test file is "whitebox" — (ie. not in the xmp_test package)
	content = bytes.Replace(content, []byte("postcards/v"+version.Version), []byte("postcards/v0.0.0"), -1)

	assert.Equal(t, testhelpers.SampleXMP, content)
}

func TestDecode(t *testing.T) {
	ex := testhelpers.SamplePostcard.Meta

	// As XMP comes from an image the secrets will have been prehidden
	ex.Front.Secrets[0].Prehidden = true
	ex.Back.Secrets[0].Prehidden = true

	// Postcard XMP isn't expected to hold size data
	ex.Physical.FrontDimensions = types.Size{}
	bnd := bundle{r: bytes.NewReader(testhelpers.SampleXMP)}

	pc, err := bnd.Decode(formats.DecodeOptions{})
	assert.NoError(t, err)

	// Because floating points are imprecise we need to compare them manually.
	// Here we iterate through the floats, and replace

	// At 254000dpi this delta equates to one pixel — so waaay finer than needed
	delta := 0.000001

	for si, s := range pc.Meta.Front.Secrets {
		for pi, p := range s.Points {
			assert.InDelta(t, ex.Front.Secrets[si].Points[pi].X, p.X, delta)
			assert.InDelta(t, ex.Front.Secrets[si].Points[pi].Y, p.Y, delta)
			pc.Meta.Front.Secrets[si].Points[pi] = ex.Front.Secrets[si].Points[pi]
		}
	}
	for si, s := range pc.Meta.Back.Secrets {
		for pi, p := range s.Points {
			assert.InDelta(t, ex.Back.Secrets[si].Points[pi].X, p.X, delta)
			assert.InDelta(t, ex.Back.Secrets[si].Points[pi].Y, p.Y, delta)
			pc.Meta.Back.Secrets[si].Points[pi] = ex.Back.Secrets[si].Points[pi]
		}
	}

	assert.Equal(t, ex, pc.Meta)
}
