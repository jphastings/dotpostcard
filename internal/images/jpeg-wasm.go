//go:build wasm
// +build wasm

package images

import (
	"bytes"
	"image"
	"io"

	"image/color"
	"image/jpeg"

	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func WriteJPEG(w io.Writer, combinedImg image.Image, xmpData []byte, bgColor color.RGBA) error {
	jpegOpts := &jpeg.Options{
		Quality: 75,
	}

	jpgData := new(bytes.Buffer)
	if err := jpeg.Encode(jpgData, WithBackgroundColor(combinedImg, bgColor), jpegOpts); err != nil {
		return err
	}

	return xmpinject.XMPintoJPEG(w, jpgData.Bytes(), xmpData)
}
