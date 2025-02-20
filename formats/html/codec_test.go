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

	assert.Equal(t, `<!-- Make sure you reference postcards.css in your <head> -->
<link rel="stylesheet" type="text/css" href="postcards.css">
<!-- You can set the width of .postcard in CSS to limit the size of all postcards on your page -->
<style>.postcard { max-width: 50vw; margin: auto; } body { margin: 1em; }</style>
<!-- Put the lines following this wherever you want your postcard -->

<input type="checkbox" id="postcard-some-postcard" style="display:none">
<label for="postcard-some-postcard" class="postcard flip-book landscape" style="--postcard: url('some-postcard.postcard.jpeg'); --aspect-ratio: 1480 / 1050">
	<img src="some-postcard.postcard.jpeg" loading="lazy" alt="The word &#39;Front&#39; in large blue letters">
	<div class="shadow"></div>
</label>`, string(content))
}
