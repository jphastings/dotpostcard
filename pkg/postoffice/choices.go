package postoffice

import (
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/css"
	"github.com/jphastings/dotpostcard/formats/html"
	"github.com/jphastings/dotpostcard/formats/usdz"
	"github.com/jphastings/dotpostcard/formats/web"
)

func DefaultCodecChoices() (CodecChoices, error) {
	webCodec, err := web.Codec("jpeg", "png")
	if err != nil {
		return nil, err
	}

	return map[string][]formats.Codec{
		"web":  {webCodec, html.Codec(), css.Codec()},
		"usdz": {usdz.Codec()},
	}, nil
}
