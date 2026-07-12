package images

import (
	"image"
	"image/color"
	"image/draw"
)

func WithBackgroundColor(img image.Image, bg color.RGBA) image.Image {
	withBg := image.NewRGBA(img.Bounds())
	draw.Draw(withBg, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	draw.Draw(withBg, img.Bounds(), img, image.Point{}, draw.Over)
	return withBg
}
