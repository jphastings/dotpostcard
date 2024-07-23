package web

import (
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/tiff"

	"github.com/jphastings/postcards/formats"
)

const codecName = "Web"

var _ formats.Bundle = bundle{}

type bundle struct {
	io.Reader
	name    string
	refPath string
}

var _ formats.Codec = codec{}

type codec struct{}

func Codec() formats.Codec { return codec{} }

func (c codec) Name() string { return codecName }
