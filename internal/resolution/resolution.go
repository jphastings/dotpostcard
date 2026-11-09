package resolution

import (
	"fmt"
	"math/big"

	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

var readers = []struct {
	magicBytes []byte
	fn         func([]byte) (*big.Rat, *big.Rat, error)
	exif       func([]byte) ([]byte, error)
}{
	{[]byte(pngHeader), decodePNG, exifFromPNG},
	{[]byte("\xff\xd8"), decodeJPEG, exifFromJPEG},
	{[]byte("RIFF????WEBPVP8"), decodeWebP, xmpinject.EXIFfromWebP},
	{[]byte{0x4D, 0x4D, 0x00, 0x2A}, decodeTIFF, exifFromTIFF},
	{[]byte{0x49, 0x49, 0x2A, 0x00}, decodeTIFF, exifFromTIFF},
}

// Decode returns the width and height resolution (number of pixels per centimetre) an image declares it is stored with
func Decode(data []byte) (*big.Rat, *big.Rat, error) {
	for _, r := range readers {
		if isMagic(data[0:len(r.magicBytes)], r.magicBytes) {
			return r.fn(data)
		}
	}
	return nil, nil, fmt.Errorf("unknown image format")
}

func isMagic(data, magic []byte) bool {
	if len(magic) != len(data) {
		return false
	}

	for i, b := range data {
		if magic[i] != b && magic[i] != '?' {
			return false
		}
	}

	return true
}

// Orientation returns the EXIF orientation (1–8) an image declares, or 1 if it declares no valid one.
func Orientation(data []byte) int {
	for _, r := range readers {
		if len(data) >= len(r.magicBytes) && isMagic(data[:len(r.magicBytes)], r.magicBytes) {
			exifData, err := r.exif(data)
			if err != nil {
				return 1
			}
			return exifOrientation(exifData)
		}
	}
	return 1
}
