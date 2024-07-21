package postcards

import (
	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/css"
	"github.com/jphastings/postcards/formats/html"
	"github.com/jphastings/postcards/formats/metadata"
	"github.com/jphastings/postcards/formats/sides"
	"github.com/jphastings/postcards/formats/xmp"
)

var Formats = map[string]formats.Codec{
	"json":  metadata.Codec(metadata.AsJSON),
	"yaml":  metadata.Codec(metadata.AsYAML),
	"sides": sides.Codec(),
	"css":   css.Codec(),
	"html":  html.Codec(),
	"xmp":   xmp.Codec(),
}
