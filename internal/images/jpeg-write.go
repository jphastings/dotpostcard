//go:build !wasm
// +build !wasm

package images

import (
	"bytes"
	"image"
	"io"

	"github.com/gen2brain/jpegli"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func WriteJPEG(w io.Writer, combinedImg image.Image, xmpData []byte) error {
	jpegliOpts := &jpegli.EncodingOptions{
		Quality:           70,
		ProgressiveLevel:  2,
		FancyDownsampling: true,
	}

	jpgData := new(bytes.Buffer)
	if err := jpegli.Encode(jpgData, combinedImg, jpegliOpts); err != nil {
		return err
	}

	return xmpinject.XMPintoJPEG(w, jpgData.Bytes(), xmpData)
}
