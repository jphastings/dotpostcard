package resolution

import (
	"math/big"

	"github.com/chai2010/webp"
)

func decodeWebP(data []byte) (*big.Rat, *big.Rat, error) {
	exif, err := webp.GetMetadata(data, "EXIF")
	if err != nil {
		return nil, nil, err
	}

	return decodeExif(exif)
}
