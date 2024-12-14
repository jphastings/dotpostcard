package images

import (
	"bytes"
	"image"
	"image/png"
	"io"

	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func WritePNG(w io.Writer, combinedImg image.Image, xmpData []byte, _ bool) error {
	pngData := new(bytes.Buffer)
	if err := png.Encode(pngData, combinedImg); err != nil {
		return err
	}

	return xmpinject.XMPintoPNG(w, pngData.Bytes(), xmpData)
}
