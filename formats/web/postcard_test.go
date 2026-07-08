package web

import (
	"bytes"
	"io"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestPostcardCodecUsesSingleExtensionFilename(t *testing.T) {
	pc := testhelpers.SamplePostcard
	// testhelpers.SamplePostcard.Front/Back are nil: fixtures.go builds that struct
	// literal from testhelpers.TestImages before TestImages is populated (that
	// happens in an init() func, which Go runs after package-level var
	// initializers). Substitute the real images here, mirroring
	// pkg/collection/helpers_test.go.
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	assert.NotNil(t, pc.Front)
	assert.NotNil(t, pc.Back)

	fws, err := PostcardCodec.Encode(pc, nil)
	assert.NoError(t, err)
	assert.Len(t, fws, 1)
	assert.Equal(t, pc.Name+".postcard", fws[0].Filename)

	data, err := fws[0].Bytes()
	assert.NoError(t, err)

	decoded, err := BundleFromReader(io.NopCloser(bytes.NewReader(data)), fws[0].Filename).Decode(formats.DecodeOptions{})
	assert.NoError(t, err)
	assert.Equal(t, pc.Name, decoded.Name)
	assert.Equal(t, pc.Meta.Sender.Name, decoded.Meta.Sender.Name)
}

func TestPostcardCodecRejectsSupportFiles(t *testing.T) {
	pc := testhelpers.SamplePostcard
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]

	_, err := PostcardCodec.Encode(pc, &formats.EncodeOptions{IncludeSupportFiles: true})
	assert.Error(t, err)
}

func TestBundleRecognisesBareExtension(t *testing.T) {
	group := testhelpers.TestFiles([]string{"some-postcard.postcard", "other.json"})

	bundles, remaining, err := DefaultCodec.Bundle(group)
	assert.NoError(t, err)
	assert.Len(t, bundles, 1)

	b, ok := bundles[0].(bundle)
	assert.True(t, ok)
	assert.Equal(t, "some-postcard", b.name)

	assert.Equal(t, []string{"other.json"}, testhelpers.Filenames(remaining))
}
