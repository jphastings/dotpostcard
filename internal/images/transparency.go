package images

import "image"

// HasTransparency reports whether the image contains genuine transparency: a
// non-trivial number of meaningfully transparent pixels. Scans sometimes
// carry a junk alpha channel where stray pixels dip slightly below opaque
// (scanner noise, export artifacts); treating those as real transparency
// would silently disable border removal.
func HasTransparency(im image.Image) bool {
	alpha := toAlpha(im)
	needed := max(16, len(alpha.Pix)/100_000)

	count := 0
	for _, a := range alpha.Pix {
		if a < 128 {
			count++
			if count >= needed {
				return true
			}
		}
	}
	return false
}
