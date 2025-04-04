package types

import (
	"fmt"
	"image/color"
	"strings"
)

type Color color.RGBA

func colorFromString(str string) (r, g, b uint8, err error) {
	str = strings.Trim(str, `"`)
	str = strings.TrimPrefix(str, `#`)

	_, err = fmt.Sscanf(str, "%02X%02X%02X", &r, &g, &b)
	return
}
