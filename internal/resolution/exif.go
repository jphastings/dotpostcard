package resolution

import (
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
	var toCm func(*big.Rat) *big.Rat

	for _, tag := range tags {
		switch tag.TagName {
		case "XResolution":
			vals := tag.Value.([]exifcommon.Rational)
			xRes = big.NewRat(int64(vals[0].Numerator), int64(vals[0].Denominator))
		case "YResolution":
			vals := tag.Value.([]exifcommon.Rational)
			yRes = big.NewRat(int64(vals[0].Numerator), int64(vals[0].Denominator))
		case "ResolutionUnit":
			toCm = ResolutionToCm(tag.Value.([]uint16)[0])
		}
	}

	// No ResolutionUnit implies there's no intended physical sizing
	if toCm == nil {
		return nil, nil, nil
	}

	return toCm(xRes), toCm(yRes), nil
}

func ResolutionToCm(exifResolutionUnit uint16) func(*big.Rat) *big.Rat {
	var toCm *big.Rat
	switch exifResolutionUnit {
	case 3:
		toCm = big.NewRat(1, 1)
	default: // 1 and 2 are both actually inches, anything else and we assume the default
		toCm = big.NewRat(100, 254)
	}

	return func(res *big.Rat) *big.Rat {
		if res == nil {
			return nil
		}
		return new(big.Rat).Mul(res, toCm)
	}
}
