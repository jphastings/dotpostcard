package resolution

import (
	"fmt"
	"math/big"

	exif "github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
)

func decodeExif(data []byte) (*big.Rat, *big.Rat, error) {
	tags, _, err := exif.GetFlatExifData(data, nil)
	if err != nil {
		return nil, nil, err
	}

	var xRes *big.Rat
	var yRes *big.Rat
	var toCm *big.Rat

	for _, tag := range tags {
		switch tag.TagName {
		case "XResolution":
			vals := tag.Value.([]exifcommon.Rational)
			xRes = big.NewRat(int64(vals[0].Numerator), int64(vals[0].Denominator))
		case "YResolution":
			vals := tag.Value.([]exifcommon.Rational)
			yRes = big.NewRat(int64(vals[0].Numerator), int64(vals[0].Denominator))
		case "ResolutionUnit":
			vals := tag.Value.([]uint16)
			switch vals[0] {
			case 3:
				toCm = big.NewRat(1, 1)
			case 1, 2:
				toCm = big.NewRat(100, 254)
			default:
				return nil, nil, fmt.Errorf("invalid Exif resolution units provided")
			}
		}
	}

	xRes.Mul(xRes, toCm)
	yRes.Mul(yRes, toCm)

	return xRes, yRes, nil
}
