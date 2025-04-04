package types

import (
	"fmt"
	"image/color"
	"strings"
)

var defaultCardColor = color.RGBA{230, 230, 217, 255}

type Color color.RGBA

func (c *Color) RGBA() color.RGBA {
	if c == nil {
		return defaultCardColor
	}
	return color.RGBA(*c)
}

func colorFromString(str string) (r, g, b uint8, err error) {
	str = strings.Trim(str, `"`)
	str = strings.TrimPrefix(str, `#`)

	_, err = fmt.Sscanf(str, "%02X%02X%02X", &r, &g, &b)
	return
}
