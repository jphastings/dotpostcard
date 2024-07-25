package web

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/jphastings/postcards/types"
)

func (b bundle) Decode() (types.Postcard, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(b, &dataCopy)

	img, _, err := image.Decode(t)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("unable to decode image: %w", err)
	}

	// xmpData, err := webp.GetMetadata(dataCopy.Bytes(), "xmp")
	// if err != nil {
	// 	return types.Postcard{}, fmt.Errorf("couldn't extract XMP metadata: %w", err)
	// }
	//
	// pc, err := xmp.BundleFromBytes(xmpData, b.refPath).Decode()
	// if err != nil {
	// 	return types.Postcard{}, fmt.Errorf("didn't contain postcard metadata: %w", err)
	// }
	pc := types.Postcard{
		Meta: types.Metadata{
			Flip: types.FlipBook,
		},
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
