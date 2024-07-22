package css_test

import (
	"io"
	"strings"
	"testing"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/css"
	"github.com/jphastings/postcards/internal/testhelpers"
	"github.com/jphastings/postcards/types"
	"github.com/stretchr/testify/assert"
)

func TestBundle(t *testing.T) {
	anyOldFilenames := []string{"any.jpg", "other.json", "files-meta.yaml"}
	files, dir := testhelpers.TestFiles(anyOldFilenames)

	bundle, remaining, errs := css.Codec().Bundle(files, dir)

	assert.Nil(t, bundle)
	assert.Equal(t, files, remaining)
	assert.Empty(t, errs)
}

func TestEncode(t *testing.T) {
	errs := make(chan error, 100)
	fws := css.Codec().Encode(types.Postcard{}, formats.EncodeOptions{}, errs)

	assert.Len(t, fws, 1)
	content, err := io.ReadAll(fws[0])
	assert.NoError(t, err)

	assert.Empty(t, errs)

	assert.True(t, strings.HasPrefix(string(content), "input[id^=postcard-] {"))
}
