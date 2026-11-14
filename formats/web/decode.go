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

// decodeParts reads the postcard's metadata and its combined image, along with the raw XMP.
func decodeParts(b bundle, decOpts formats.DecodeOptions) (pc types.Postcard, img image.Image, xmpData []byte, err error) {
	img, xmpData, err = images.Decode(b)
	if err != nil {
		return pc, nil, nil, fmt.Errorf("unable to decode image: %w", err)
	}
	if len(xmpData) == 0 {
		return pc, nil, nil, fmt.Errorf("image didn't contain XMP metadata, it's not readable as a postcard")
	}

	pc, err = xmp.BundleFromBytes(xmpData, b.refPath).Decode(decOpts)
	if err != nil {
		return pc, nil, nil, fmt.Errorf("image didn't contain postcard metadata: %w", err)
	}
	pc.Name = b.name

	pc.Meta.HasTransparency = images.HasTransparency(img) && !decOpts.IgnoreTransparency
	return pc, img, xmpData, nil
}

func (b bundle) Decode(decOpts formats.DecodeOptions) (types.Postcard, error) {
	defer b.Close()

	pc, img, _, err := decodeParts(b, decOpts)
	if err != nil {
		return types.Postcard{}, err
	}

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

	pc.Back = turnBack(back, -webTurns(pc.Meta.Flip))

	return pc, nil
}
