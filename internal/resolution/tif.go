package resolution

import (
	"math/big"

	tiffstructure "github.com/dsoprea/go-tiff-image-structure"
)

func decodeTIFF(data []byte) (*big.Rat, *big.Rat, error) {
	jmp := tiffstructure.NewTiffMediaParser()

	mc, err := jmp.ParseBytes(data)
	if err != nil {
		return nil, nil, err
	}

	_, exifData, err := mc.Exif()
	if err != nil {
		return nil, nil, err
	}

	return decodeExif(exifData)
}
