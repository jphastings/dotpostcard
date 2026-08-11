package postcards

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/stretchr/testify/assert"
)

func TestExistingOutputsReportsCollision(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sample.postcard.jpeg")
	assert.NoError(t, os.WriteFile(target, []byte("already here"), 0644))

	bundle := web.BundleFromReader(io.NopCloser(strings.NewReader("")), "sample.postcard.jpeg")

	existing := ExistingOutputs(bundle, []formats.Codec{web.DefaultCodec}, &formats.EncodeOptions{}, dir)
	assert.Equal(t, []string{target}, existing)
}

func TestExistingOutputsEmptyDirReportsNothing(t *testing.T) {
	dir := t.TempDir()
	bundle := web.BundleFromReader(io.NopCloser(strings.NewReader("")), "sample.postcard.jpeg")

	existing := ExistingOutputs(bundle, []formats.Codec{web.DefaultCodec}, &formats.EncodeOptions{}, dir)
	assert.Empty(t, existing)
}

func TestExistingOutputsUnknownCardNameReportsNothing(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "sample.postcard.jpeg"), []byte("already here"), 0644))

	// A bundle with no filesystem origin (eg. postoffice, xmp.BundleFromBytes with an
	// empty refPath) has no CardName to predict against.
	bundle := xmp.BundleFromBytes([]byte{}, "")

	existing := ExistingOutputs(bundle, []formats.Codec{web.DefaultCodec}, &formats.EncodeOptions{}, dir)
	assert.Empty(t, existing)
}
