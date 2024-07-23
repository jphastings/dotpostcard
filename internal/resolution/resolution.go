package resolution

import (
	"fmt"
	"math/big"
)

var readers = []struct {
	magicBytes []byte
	fn         func([]byte) (*big.Rat, *big.Rat, error)
}{
	{[]byte(pngHeader), decodePNG},
	{[]byte("\xff\xd8"), decodeJPEG},
	{[]byte("RIFF????WEBPVP8"), decodeWebP},
	{[]byte{0x4D, 0x4D, 0x00, 0x2A}, decodeTIFF},
	{[]byte{0x49, 0x49, 0x2A, 0x00}, decodeTIFF},
}

// Decode returns the resolution (number of pixels per centimetre) an image declares it is stored with
func Decode(data []byte) (*big.Rat, *big.Rat, error) {
	for _, r := range readers {
		if isMagic(data[0:len(r.magicBytes)], r.magicBytes) {
			return r.fn(data)
		}
	}
	return nil, nil, fmt.Errorf("unparseable image format")
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
