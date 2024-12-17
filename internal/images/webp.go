package images

import (
	"bytes"
	"image"
	"io"

	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func ReadWebP(r io.Reader) (image.Image, []byte, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(r, &dataCopy)

	img, err := decodeWebP(t)
	if err != nil {
		return nil, nil, err
	}

	if xmpData, err := xmpinject.XMPfromWebP(dataCopy.Bytes()); err == nil {
		return img, xmpData, nil
	}

	return img, nil, nil
}
