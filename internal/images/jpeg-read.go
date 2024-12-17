package images

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"

	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func ReadJPEG(r io.Reader) (image.Image, []byte, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(r, &dataCopy)

	img, err := jpeg.Decode(t)
	if err != nil {
		return nil, nil, err
	}

	if xmpData, err := xmpinject.XMPfromJPEG(dataCopy.Bytes()); err == nil {
		return img, xmpData, nil
	}

	return img, nil, nil
}
