package postcards

import (
	"fmt"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/component"
	"github.com/jphastings/postcards/formats/css"
	"github.com/jphastings/postcards/formats/html"
	"github.com/jphastings/postcards/formats/metadata"
	"github.com/jphastings/postcards/formats/web"
	"github.com/jphastings/postcards/formats/xmp"
)

var codecs = map[string]formats.Codec{
	"component": component.Codec(),
	"web":       web.Codec(),
	"json":      metadata.Codec(metadata.AsJSON),
	"yaml":      metadata.Codec(metadata.AsYAML),
	"css":       css.Codec(),
	"html":      html.Codec(),
	"xmp":       xmp.Codec(),
}

var codecOrder = []string{"component", "web", "json", "yaml", "css", "html", "xmp"}

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
			return nil, fmt.Errorf("the format '%s' isn' registered", name)
		}

		outCodecs = append(outCodecs, codec)
	}

	return outCodecs, nil
}
