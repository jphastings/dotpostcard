package web

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/jphastings/postcards/types"
)

func (b bundle) Decode() (types.Postcard, error) {
	pc := b.postcard

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
