package web

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/types"
)

func (b bundle) Decode(decOpts formats.DecodeOptions) (types.Postcard, error) {
	defer b.Close()

	img, xmpData, err := images.Decode(b)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("unable to decode image: %w", err)
	}
	if len(xmpData) == 0 {
		return types.Postcard{}, fmt.Errorf("image didn't contain XMP metadata, it's not readable as a postcard")
	}

	pc, err := xmp.BundleFromBytes(xmpData, b.refPath).Decode(decOpts)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("image didn't contain postcard metadata: %w", err)
	}
	pc.Name = b.name

	// TODO: migrate this into internal/images/decode.go
	_, _, _, a := img.At(0, 0).RGBA()
	pc.Meta.HasTransparency = (a != 65535) && !decOpts.IgnoreTransparency

	if pc.Meta.Flip == types.FlipNone {
		pc.Front = img
		return pc, nil
	}

	bounds := img.Bounds()
	sideW := bounds.Dx()
	sideH := bounds.Dy() / 2

	frontBounds := image.Rectangle{
		Min: image.Point{0, 0},
		Max: image.Point{sideW, sideH},
	}

	front := image.NewRGBA(image.Rect(0, 0, sideW, sideH))
	pc.Front = front
	draw.Draw(front, frontBounds, img, image.Point{}, draw.Src)

	back := image.NewRGBA(image.Rect(0, 0, sideW, sideH))
	draw.Draw(back, frontBounds, img, image.Point{0, sideH}, draw.Src)

	if pc.Meta.Flip == types.FlipLeftHand {
		// Use the opposite flip to return to the right orientation
		pc.Back, _ = rotateForWeb(back, types.FlipRightHand)
	} else if pc.Meta.Flip == types.FlipRightHand {
		// Use the opposite flip to return to the right orientation
		pc.Back, _ = rotateForWeb(back, types.FlipLeftHand)
	} else {
		pc.Back = back
	}

	return pc, nil
}
