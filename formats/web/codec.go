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
}

type capabilities struct {
	lossless     bool
	transparency bool
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

var DefaultCodec, _ = Codec("jpeg", "png")

func (c codec) Name() string { return codecName }
