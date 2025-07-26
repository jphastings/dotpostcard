package postcards

import (
	"embed"
	"fmt"
	"slices"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/component"
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
	"svg":       web.SVGCodec,
	"usd":       usd.Codec(),
	"usdz":      usdz.Codec(),
	"json":      metadata.Codec(metadata.AsJSON),
	"yaml":      metadata.Codec(metadata.AsYAML),
	"xmp":       xmp.Codec(),
}

// Used for ordering
var Codecs = []string{"component", "web", "svg", "usdz", "usd", "json", "yaml", "xmp"}

// These 'formats' will trigger the IncludeSupportFiles encoder option instead of a different codec
var supportFiles = []string{"css", "html"}

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

func CodecsByFormat(names []string) ([]formats.Codec, bool, error) {
	var outCodecs []formats.Codec
	var incSupportFiles bool

	for _, name := range names {
		if slices.Contains(supportFiles, name) {
			incSupportFiles = true
			continue
		}

		codec, ok := codecs[name]
		if !ok {
			return nil, false, fmt.Errorf("the format '%s' isn't one of those available: %s", name, strings.Join(Codecs, ", "))
		}

		outCodecs = append(outCodecs, codec)
	}

	return outCodecs, incSupportFiles, nil
}

// Returns markdown docs for the named format
func FormatDocs(name string) (string, error) {
	b, err := formatDocs.ReadFile(fmt.Sprintf("docs/formats/%s.md", name))
	return string(b), err
}
