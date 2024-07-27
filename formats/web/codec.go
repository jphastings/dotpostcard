package web

import (
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/jphastings/dotpostcard/formats"
)

const codecName = "Web"

var _ formats.Bundle = bundle{}

type bundle struct {
	io.Reader
	name    string
	refPath string
}

var _ formats.Codec = codec{}

type codec struct {
	format string
}

func Codec(format string) formats.Codec { return codec{format: format} }

func (c codec) Name() string { return codecName }
