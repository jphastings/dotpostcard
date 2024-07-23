package resolution

import (
	"fmt"
	"math/big"

	exif "github.com/dsoprea/go-exif/v3"
)

const (
	resUnitCode = "Exif.Image.ResolutionUnit"
	resXCode    = "Exif.Image.XResolution"
	resYCode    = "Exif.Image.YResolution"
)

func decodeExif(data []byte) (*big.Rat, *big.Rat, error) {
	tags, med, err := exif.GetFlatExifData(data, nil)
	if err != nil {
		return nil, nil, err
	}
	fmt.Println(tags, med)

	return nil, nil, fmt.Errorf("exif scanning not implemented")
}

// func GetExifResolution(im *goexiv.Image) (*big.Rat, *big.Rat, error) {
// 	toCm, err := getExifResUnit(im)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	xRes, xErr := getExifResCount(im, resXCode)
// 	yRes, yErr := getExifResCount(im, resYCode)

// 	return toCm(xRes), toCm(yRes), errors.Join(xErr, yErr)
// }

// func getExifResUnit(im *goexiv.Image) (func(*big.Rat) *big.Rat, error) {
// 	unit, err := im.GetExifData().GetString(resUnitCode)
// 	if err != nil {
// 		return nil, err
// 	}

// 	switch unit {
// 	case "3": // in centimeters
// 		return func(r *big.Rat) *big.Rat { return r }, nil
// 	case "2", "1": // in inches
// 		return func(r *big.Rat) *big.Rat {
// 			return big.NewRat(1, 1).Quo(r, big.NewRat(254, 100))
// 		}, nil
// 	default:
// 		return nil, fmt.Errorf("unknown EXIF resolution unit: %v", unit)
// 	}
// }

// func getExifResCount(im *goexiv.Image, tag string) (*big.Rat, error) {
// 	val, err := im.GetExifData().GetString(tag)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var a, b int64
// 	if _, err := fmt.Sscanf(val, "%d/%d", &a, &b); err != nil {
// 		return nil, fmt.Errorf("invalid rational number format: %w", err)
// 	}

// 	return big.NewRat(a, b), nil
// }
