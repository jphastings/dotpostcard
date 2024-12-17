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

func ReadPNG(r io.Reader) (image.Image, []byte, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(r, &dataCopy)

	img, err := png.Decode(t)
	if err != nil {
		return nil, nil, err
	}

	if xmpData, err := xmpinject.XMPfromPNG(dataCopy.Bytes()); err == nil {
		return img, xmpData, nil
	}

	return img, nil, nil
}
