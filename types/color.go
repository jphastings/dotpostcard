package types

import (
	"fmt"
	"image/color"
	"strings"
)

type Color color.RGBA

func ColorFromString(str string) (*Color, error) {
	r, g, b, err := rgbFromString(str)
	if err != nil {
		return nil, err
	}

	return &Color{
		R: r,
		G: g,
		B: b,
		A: 0xff,
	}, nil
}

func rgbFromString(str string) (r, g, b uint8, err error) {
	str = strings.Trim(str, `"`)
	str = strings.TrimPrefix(str, `#`)

	_, err = fmt.Sscanf(str, "%02X%02X%02X", &r, &g, &b)
	return
}

func (c *Color) String() string {
	if c == nil {
		return ""
	}

	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
