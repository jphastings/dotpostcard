package postcards

import (
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

var codecs = map[string]formats.Codec{
	"component": component.Codec(),
	"web":       web.Codec("webp"),
	"usd":       usd.Codec(),
	"usdz":      usdz.Codec(),
	"json":      metadata.Codec(metadata.AsJSON),
	"yaml":      metadata.Codec(metadata.AsYAML),
	"css":       css.Codec(),
	"html":      html.Codec(),
	"xmp":       xmp.Codec(),
}

var codecOrder = []string{"component", "web", "usdz", "usd", "json", "yaml", "css", "html", "xmp"}

func init() {
	if len(codecOrder) != len(codecs) {
		panic("Codec order count doesn't match codec mapping")
	}
	for _, c := range codecOrder {
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
			return nil, fmt.Errorf("the format '%s' isn't one of those available: %s", name, Formats())
		}

		outCodecs = append(outCodecs, codec)
	}

	return outCodecs, nil
}

func Formats() string {
	return strings.Join(codecOrder, ", ")
}
