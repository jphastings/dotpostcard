package collection

import (
	"testing"

	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestAddWebPostcardAcceptsSingleExtensionFilename(t *testing.T) {
	pc := testhelpers.SamplePostcard
	pc.Name = "single-ext-postcard"
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	assert.NotNil(t, pc.Front)
	assert.NotNil(t, pc.Back)

	fws, err := web.PostcardCodec.Encode(pc, nil)
	assert.NoError(t, err)
	assert.Len(t, fws, 1)
	assert.Equal(t, "single-ext-postcard.postcard", fws[0].Filename)

	data, err := fws[0].Bytes()
	assert.NoError(t, err)

	col := mustCreate(t)
	summary, err := col.AddWebPostcard(fws[0].Filename, data)
	assert.NoError(t, err)

	assert.Equal(t, "single-ext-postcard", summary.Name)
	assert.Equal(t, "single-ext-postcard.postcard", summary.Filename)
	assert.Equal(t, "image/jpeg", summary.Mimetype)
}

func TestMimetypeFromData(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	png := []byte("\x89PNG\r\n\x1a\nrest-of-file")
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	unknown := []byte("not an image")

	mimetype, err := MimetypeFromData(jpeg)
	assert.NoError(t, err)
	assert.Equal(t, "image/jpeg", mimetype)

	mimetype, err = MimetypeFromData(png)
	assert.NoError(t, err)
	assert.Equal(t, "image/png", mimetype)

	mimetype, err = MimetypeFromData(webp)
	assert.NoError(t, err)
	assert.Equal(t, "image/webp", mimetype)

	_, err = MimetypeFromData(unknown)
	assert.Error(t, err)
}
