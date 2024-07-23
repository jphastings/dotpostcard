package web

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/jphastings/postcards/types"
)

func (b bundle) Decode() (types.Postcard, error) {
	pc := types.Postcard{}
	// TODO: decode XMP

	// xmpData, err := webp.GetMetadata(data, "xmp")
	// if err != nil {
	// 	finalErr = errors.Join(finalErr, formats.NewFileError(
	// 		filename,
	// 		fmt.Errorf("couldn't read file to determine if it is a postcard image: %w", err),
	// 	))
	// 	continue
	// }

	// // May as well keep/use the pre-decoded metadata as the basis for the postcard later (rather than re-reading/processing)
	// pc, err := xmp.BundleFromBytes(xmpData).Decode()
	// if err != nil {
	// 	//
	// 	// finalErr = errors.Join(finalErr, formats.NewFileError(
	// 	// 	filename,
	// 	// 	fmt.Errorf("didn't contain postcard metadata: %w", err),
	// 	// ))
	// 	continue
	// }

	img, _, err := image.Decode(b)
	if err != nil {
		return pc, fmt.Errorf("unable to decode image: %w", err)
	}

	if pc.Meta.Flip == types.FlipNone {
		pc.Front = img
		return pc, nil
	}

	sideW := pc.Meta.FrontDimensions.PxWidth
	sideH := pc.Meta.FrontDimensions.PxHeight / 2

	frontBounds := image.Rectangle{
		Min: image.Point{0, 0},
		Max: image.Point{sideW, sideH},
	}
	backBounds := image.Rectangle{
		Min: image.Point{0, sideH},
		Max: image.Point{sideW, sideH * 2},
	}

	front := image.NewRGBA(image.Rect(0, 0, sideW, sideH))
	pc.Front = front
	draw.Draw(front, frontBounds, img, image.Point{}, draw.Src)

	back := image.NewRGBA(image.Rect(0, 0, sideW, sideH))
	pc.Back = back
	draw.Draw(back, backBounds, img, image.Point{}, draw.Src)

	return pc, nil
}
