package images

import (
	"bytes"
	"encoding/binary"
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

// IsLosslessWebP reports whether the WebP's image chunk is lossless (VP8L), whether or not the
// file is wrapped in an extended (VP8X) container.
func IsLosslessWebP(data []byte) bool {
	for i := 12; i+8 <= len(data); {
		switch string(data[i : i+4]) {
		case "VP8L":
			return true
		case "VP8 ":
			return false
		}
		i += 8 + int(binary.LittleEndian.Uint32(data[i+4:i+8]))
		i += i % 2
	}
	return false
}
