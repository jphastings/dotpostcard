package web

import (
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/stretchr/testify/assert"
)

func TestOutputNamesDefaultOffersBothFormats(t *testing.T) {
	names := DefaultCodec.OutputNames("sample", &formats.EncodeOptions{})
	assert.ElementsMatch(t, []string{"sample.postcard.jpeg", "sample.postcard.webp"}, names)
}

func TestOutputNamesArchivalIsLosslessOnly(t *testing.T) {
	names := DefaultCodec.OutputNames("sample", &formats.EncodeOptions{Archival: true})
	assert.Equal(t, []string{"sample.postcard.webp"}, names)
}

func TestOutputNamesIgnoreTransparencyIsLossyOnly(t *testing.T) {
	names := DefaultCodec.OutputNames("sample", &formats.EncodeOptions{NoTransparency: true})
	assert.Equal(t, []string{"sample.postcard.jpeg"}, names)
}

func TestOutputNamesSingleExtCodecUsesBareName(t *testing.T) {
	names := PostcardCodec.OutputNames("sample", &formats.EncodeOptions{})
	assert.Equal(t, []string{"sample.postcard"}, names)
}

func TestOutputNamesIncludesHTMLButNotSharedCSS(t *testing.T) {
	names := DefaultCodec.OutputNames("sample", &formats.EncodeOptions{IncludeSupportFiles: true})
	assert.Contains(t, names, "sample.html")
	assert.NotContains(t, names, "postcards.css", "postcards.css is shared across cards and must not make every card after the first look already-converted")
}
