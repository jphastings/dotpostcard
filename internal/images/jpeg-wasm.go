//go:build wasm
// +build wasm

package images

import (
	"bytes"
	"image"
	"io"

	"image/color"
	"image/jpeg"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

const (
	efficientJPEGQuality = 75
	detailJPEGQuality    = 90
)

func WriteJPEG(w io.Writer, combinedImg image.Image, xmpData []byte, bgColor color.RGBA, quality formats.Quality) error {
	jpegQuality := efficientJPEGQuality
	if quality == formats.Detail {
		jpegQuality = detailJPEGQuality
	}
	jpegOpts := &jpeg.Options{Quality: jpegQuality}

	jpgData := new(bytes.Buffer)
	if err := jpeg.Encode(jpgData, WithBackgroundColor(combinedImg, bgColor), jpegOpts); err != nil {
		return err
	}

	return xmpinject.XMPintoJPEG(w, jpgData.Bytes(), xmpData)
}
