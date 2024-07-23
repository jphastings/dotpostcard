package web

import (
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/tiff"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
)

var _ formats.Bundle = bundle{}

type bundle struct {
	referenceFilename string

	io.Reader
	postcard types.Postcard
}

var _ formats.Codec = codec{}

type codec struct{}

func Codec() formats.Codec { return codec{} }
