//go:build !wasm
// +build !wasm

package images

import (
	"bytes"
	"image"
	"image/color"
	"io"

	"github.com/gen2brain/jpegli"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

const (
	efficientJPEGQuality = 70
	detailJPEGQuality    = 90
)

func WriteJPEG(w io.Writer, combinedImg image.Image, xmpData []byte, bgColor color.RGBA, quality formats.Quality) error {
	jpegQuality := efficientJPEGQuality
	if quality == formats.Detail {
		jpegQuality = detailJPEGQuality
	}
	jpegliOpts := &jpegli.EncodingOptions{
		Quality:           jpegQuality,
		ProgressiveLevel:  2,
		FancyDownsampling: true,
	}

	jpgData := new(bytes.Buffer)
	if err := jpegli.Encode(jpgData, WithBackgroundColor(combinedImg, bgColor), jpegliOpts); err != nil {
		return err
	}

	return xmpinject.XMPintoJPEG(w, jpgData.Bytes(), xmpData)
}
