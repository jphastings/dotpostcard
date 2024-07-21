package postcards

import (
	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/css"
	"github.com/jphastings/postcards/formats/html"
	"github.com/jphastings/postcards/formats/metadata"
)

var Formats = map[string]formats.Codec{
	"json": metadata.Codec(metadata.AsJSON),
	"yaml": metadata.Codec(metadata.AsYAML),
	"css":  css.Codec(),
	"html": html.Codec(),
}
