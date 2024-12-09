package html_test

import (
	"testing"

	"github.com/jphastings/dotpostcard/formats/html"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestBundle(t *testing.T) {
	anyOldFilenames := []string{"any.jpg", "other.json", "files-meta.yaml"}
	group := testhelpers.TestFiles(anyOldFilenames)

	bundle, remaining, errs := html.Codec().Bundle(group)

	assert.Nil(t, bundle)
	assert.Equal(t, group.Files, remaining)
	assert.Empty(t, errs)
}

func TestEncode(t *testing.T) {
	fws, err := html.Codec().Encode(testhelpers.SamplePostcard, nil)
	assert.NoError(t, err)

	assert.Len(t, fws, 1)

	content, err := fws[0].Bytes()
	assert.NoError(t, err)

	assert.Equal(t, `<link rel="stylesheet" type="text/css" href="postcards.css">
<div style="max-width:50vw;margin: auto;">

<input type="checkbox" id="postcard-some-postcard">
<label for="postcard-some-postcard">
	<div class="postcard flip-book landscape" style="--postcard: url('some-postcard.postcard.jpg'); --aspect-ratio: 1480 / 1050">
		<img src="some-postcard.postcard.jpg" loading="lazy" alt="The word &#39;Front&#39; in large blue letters" width="500px">
		<div class="shadow"></div>
	</div>
</label>`, string(content))
}
