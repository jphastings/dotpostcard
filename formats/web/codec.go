package web

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/jphastings/dotpostcard/formats"
)

const codecName = "Web"

var _ formats.Bundle = bundle{}

type bundle struct {
	io.ReadCloser
	name    string
	refPath string
}

var _ formats.Codec = codec{}

type codec struct {
	// This holds a list of formats, the first will be tried, and if it's unsuitable, the next etc.
	// This is particularly useful for transparency. Eg. A `web.Codec("jpeg", "webp")` would save as jpg
	// if there is no transparency, and WebP if there is transparency to encode, or if "archival" was
	// specified (as JPEG can't encode images losslessly).
	formats []string
	// When true, the encoder omits the trailing image-format extension so the file is named
	// `{name}.postcard`, rather than `{name}.postcard.{ext}`.
	singleExt bool
}

type capabilities struct {
	lossless     bool
	transparency bool
	masking      bool
}

func (c capabilities) String() string {
	desc := ""

	if c.lossless {
		desc += "lossless, "
	}
	if c.transparency {
		desc += "transparency, "
	}

	if desc == "" {
		desc = "no constraints"
	} else {
		desc = desc[:len(desc)-2]
	}

	return desc
}

var formatCapabilities = map[string]capabilities{
	"jpeg": {},
	"webp": {lossless: true, transparency: true},
	"png":  {lossless: true, transparency: true},
	"svg":  {transparency: true, masking: true},
}

// Only returns true if the capabilities on struct owning this method meet the needs of the provided capabilities object.
func meetsNeeds(format string, needs capabilities) bool {
	c, ok := formatCapabilities[format]
	return ok && (!needs.lossless || c.lossless) && (!needs.transparency || c.transparency)
}

func Codec(format string, altFormats ...string) (formats.Codec, error) {
	fmts := append([]string{format}, altFormats...)

	for _, f := range fmts {
		if _, ok := formatCapabilities[f]; !ok {
			return nil, fmt.Errorf("the format %s is not known", f)
		}
	}

	return codec{formats: fmts}, nil
}

// SingleExtCodec behaves exactly like Codec, but the returned codec names its output
// file `{name}.postcard` instead of `{name}.postcard.{ext}`. This suits contexts (like
// macOS QuickLook) where a real image extension would let the OS override the preview.
func SingleExtCodec(format string, altFormats ...string) (formats.Codec, error) {
	fmts := append([]string{format}, altFormats...)

	for _, f := range fmts {
		if _, ok := formatCapabilities[f]; !ok {
			return nil, fmt.Errorf("the format %s is not known", f)
		}
	}

	return codec{formats: fmts, singleExt: true}, nil
}

var SVGCodec, _ = Codec("svg")

func (c codec) Name() string { return codecName }
