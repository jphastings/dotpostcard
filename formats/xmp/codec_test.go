package xmp

import (
	"bytes"
	"io"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
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

	assert.Equal(t, string(testhelpers.SampleXMP), string(content))
}

func TestDecode(t *testing.T) {
	ex := testhelpers.SamplePostcard
	bnd := bundle{r: bytes.NewReader(testhelpers.SampleXMP)}

	pc, err := bnd.Decode(nil)
	assert.NoError(t, err)

	assert.Equal(t, ex.Meta, pc.Meta)
}
