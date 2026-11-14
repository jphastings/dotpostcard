//go:build wasm
// +build wasm

package images

import (
	"bytes"
	"image"

	"image/color"
	"image/jpeg"
)

const (
	efficientJPEGQuality = 75
	detailJPEGQuality    = 90
)

func encodeJPEG(img image.Image, bgColor color.RGBA, quality int) ([]byte, error) {
	jpgData := new(bytes.Buffer)
	err := jpeg.Encode(jpgData, WithBackgroundColor(img, bgColor), &jpeg.Options{Quality: quality})
	return jpgData.Bytes(), err
}
