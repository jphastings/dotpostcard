package resolution

import (
	"math/big"

	jpgstructure "github.com/dsoprea/go-jpeg-image-structure"
)

func decodeJPEG(data []byte) (*big.Rat, *big.Rat, error) {
	exifData, err := exifFromJPEG(data)
	if err != nil {
		return nil, nil, err
	}

	return decodeExif(exifData)
}

func exifFromJPEG(data []byte) ([]byte, error) {
	jmp := jpgstructure.NewJpegMediaParser()

	mc, err := jmp.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	_, exifData, err := mc.Exif()
	return exifData, err
}
