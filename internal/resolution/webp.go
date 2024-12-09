package resolution

import (
	"math/big"

	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func decodeWebP(data []byte) (*big.Rat, *big.Rat, error) {
	// TODO: Should I also check XMP data here?
	exif, err := xmpinject.EXIFfromWebP(data)
	if err != nil {
		return nil, nil, err
	}
	return decodeExif(exif)
}
