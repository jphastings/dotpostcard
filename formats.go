package postcards

import (
	"fmt"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/css"
	"github.com/jphastings/postcards/formats/html"
	"github.com/jphastings/postcards/formats/metadata"
	"github.com/jphastings/postcards/formats/raw"
	"github.com/jphastings/postcards/formats/web"
	"github.com/jphastings/postcards/formats/xmp"
)

var Formats = map[string]formats.Codec{
	"raw":  raw.Codec(),
	"web":  web.Codec(),
	"json": metadata.Codec(metadata.AsJSON),
	"yaml": metadata.Codec(metadata.AsYAML),
	"css":  css.Codec(),
	"html": html.Codec(),
	"xmp":  xmp.Codec(),
}

func CodecsByFormat(names []string) ([]formats.Codec, error) {
	var codecs []formats.Codec

	for _, name := range names {
		codec, ok := Formats[name]
		if !ok {
			return nil, fmt.Errorf("the format '%s' isn' registered", name)
		}

		codecs = append(codecs, codec)
	}

	return codecs, nil
}
