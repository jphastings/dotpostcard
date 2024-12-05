package css_test

import (
	"strings"
	"testing"

	"github.com/jphastings/dotpostcard/formats/css"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
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
	fws, err := css.Codec().Encode(types.Postcard{}, nil)
	assert.NoError(t, err)

	assert.Len(t, fws, 1)

	content, err := fws[0].Bytes()
	assert.NoError(t, err)

	assert.True(t, strings.HasPrefix(string(content), "input[id^=postcard-] {"))
}
