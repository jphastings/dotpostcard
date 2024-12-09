package postcards

import (
	"embed"
	"fmt"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/component"
	"github.com/jphastings/dotpostcard/formats/css"
	"github.com/jphastings/dotpostcard/formats/html"
	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/jphastings/dotpostcard/formats/usd"
	"github.com/jphastings/dotpostcard/formats/usdz"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/formats/xmp"
)

//go:embed docs/formats/*.md
var formatDocs embed.FS

var codecs = map[string]formats.Codec{
	"component": component.Codec(),
	"web":       web.DefaultCodec,
	"usd":       usd.Codec(),
	"usdz":      usdz.Codec(),
	"json":      metadata.Codec(metadata.AsJSON),
	"yaml":      metadata.Codec(metadata.AsYAML),
	"css":       css.Codec(),
	"html":      html.Codec(),
	"xmp":       xmp.Codec(),
}

var Codecs = []string{"component", "web", "usdz", "usd", "json", "yaml", "css", "html", "xmp"}

func init() {
	if len(Codecs) != len(codecs) {
		panic("Codec order count doesn't match codec mapping")
	}
	for _, c := range Codecs {
		if _, ok := codecs[c]; !ok {
			panic(fmt.Sprintf("the %s format name is not a registered codec", c))
		}
	}
}

func CodecsByFormat(names []string) ([]formats.Codec, error) {
	var outCodecs []formats.Codec

	for _, name := range names {
		codec, ok := codecs[name]
		if !ok {
			return nil, fmt.Errorf("the format '%s' isn't one of those available: %s", name, strings.Join(Codecs, ", "))
		}

		outCodecs = append(outCodecs, codec)
	}

	return outCodecs, nil
}

// Returns markdown docs for the named format
func FormatDocs(name string) (string, error) {
	b, err := formatDocs.ReadFile(fmt.Sprintf("docs/formats/%s.md", name))
	return string(b), err
}
