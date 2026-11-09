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

	seen := map[string]bool{}
	for _, tag := range tags {
		// IFD0 (the main image) is visited before IFD1 (the thumbnail), so the first of each tag wins
		if seen[tag.TagName] {
			continue
		}
		seen[tag.TagName] = true

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

func exifOrientation(data []byte) int {
	tags, _, err := exif.GetFlatExifData(data, nil)
	if err != nil {
		return 1
	}

	for _, tag := range tags {
		if tag.TagName != "Orientation" {
			continue
		}
		// IFD0 is visited before the thumbnail's IFD1, so the first one describes the main image
		vals, ok := tag.Value.([]uint16)
		if !ok || len(vals) == 0 || vals[0] < 1 || vals[0] > 8 {
			return 1
		}
		return int(vals[0])
	}
	return 1
}
