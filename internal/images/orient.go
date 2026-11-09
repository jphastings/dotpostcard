package images

import (
	"image"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Orient returns img as it should be displayed under the given EXIF orientation (1–8).
func Orient(img image.Image, orientation int) image.Image {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	storedToDisplayed := map[int]f64.Aff3{
		2: {-1, 0, w, 0, 1, 0},
		3: {-1, 0, w, 0, -1, h},
		4: {1, 0, 0, 0, -1, h},
		5: {0, 1, 0, 1, 0, 0},
		6: {0, -1, h, 1, 0, 0},
		7: {0, -1, h, -1, 0, w},
		8: {0, 1, 0, -1, 0, w},
	}
	s2d, ok := storedToDisplayed[orientation]
	if !ok {
		return img
	}
	dw, dh := b.Dx(), b.Dy()
	if orientation >= 5 {
		dw, dh = dh, dw
	}
	// NRGBA rather than premultiplied RGBA, which would damage soft-alpha edges
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	draw.NearestNeighbor.Transform(dst, s2d, img, b, draw.Src, nil)
	return dst
}
