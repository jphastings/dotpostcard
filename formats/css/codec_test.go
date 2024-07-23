package css_test

import (
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
	group := testhelpers.TestFiles(anyOldFilenames)

	bundle, remaining, errs := css.Codec().Bundle(group)

	assert.Nil(t, bundle)
	assert.Equal(t, group.Files, remaining)
	assert.Empty(t, errs)
}

func TestEncode(t *testing.T) {
	fws := css.Codec().Encode(types.Postcard{}, formats.EncodeOptions{})

	assert.Len(t, fws, 1)

	content, err := fws[0].Bytes()
	assert.NoError(t, err)

	assert.True(t, strings.HasPrefix(string(content), "input[id^=postcard-] {"))
}
