package postoffice

import (
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/usdz"
	"github.com/jphastings/dotpostcard/formats/web"
)

func DefaultCodecChoices() (CodecChoices, error) {
	return map[string][]formats.Codec{
		"web":  {web.DefaultCodec},
		"usdz": {usdz.Codec()},
	}, nil
}
