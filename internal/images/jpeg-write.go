//go:build !wasm
// +build !wasm

package images

import (
	"bytes"
	"image"
	"image/color"

	"github.com/gen2brain/jpegli"
)

const (
	efficientJPEGQuality = 70
	detailJPEGQuality    = 90
)

func encodeJPEG(img image.Image, bgColor color.RGBA, quality int) ([]byte, error) {
	jpegliOpts := &jpegli.EncodingOptions{
		Quality:           quality,
		ProgressiveLevel:  2,
		FancyDownsampling: true,
	}

	jpgData := new(bytes.Buffer)
	err := jpegli.Encode(jpgData, WithBackgroundColor(img, bgColor), jpegliOpts)
	return jpgData.Bytes(), err
}
