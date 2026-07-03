package images

import "image"

// HasTransparency reports whether the image contains genuine transparency.
// Scanner and export junk alpha comes as scattered speckle or thin slivers;
// genuine transparency — a removed background, even a small die-cut corner —
// always has a solid core. A pixel only counts toward transparency if its
// whole 5×5 neighbourhood is meaningfully transparent, and a non-trivial
// number of such core pixels is required.
func HasTransparency(im image.Image) bool {
	alpha := toAlpha(im)
	w, h := alpha.Rect.Dx(), alpha.Rect.Dy()
	needed := max(16, w*h/100_000)

	count := 0
	for y := 2; y < h-2; y++ {
		row := y * alpha.Stride
		for x := 2; x < w-2; x++ {
			if alpha.Pix[row+x] >= 128 || !coreTransparent(alpha, x, y) {
				continue
			}
			count++
			if count >= needed {
				return true
			}
		}
	}
	return false
}

func coreTransparent(alpha *image.Alpha, x, y int) bool {
	for dy := -2; dy <= 2; dy++ {
		row := (y + dy) * alpha.Stride
		for dx := -2; dx <= 2; dx++ {
			if alpha.Pix[row+x+dx] >= 128 {
				return false
			}
		}
	}
	return true
}
