package matting

// Trimap classifies every pixel as definite foreground, definite background,
// or part of the unknown band around the mask boundary where alpha must be
// estimated.
type Trimap struct {
	W, H int
	FG   []bool
	BG   []bool
	Band []bool
}

// BuildTrimap splits pixels around the boundary of mask into an unknown band
// of ±bandHalfWidth pixels, with the remainder classified by the mask.
func BuildTrimap(mask []bool, w, h, bandHalfWidth int) Trimap {
	dist := distanceToBoundary(mask, w, h)
	limit := int32(bandHalfWidth * chamferScale)

	tm := Trimap{
		W:    w,
		H:    h,
		FG:   make([]bool, w*h),
		BG:   make([]bool, w*h),
		Band: make([]bool, w*h),
	}
	for i, d := range dist {
		switch {
		case d <= limit:
			tm.Band[i] = true
		case mask[i]:
			tm.FG[i] = true
		default:
			tm.BG[i] = true
		}
	}
	return tm
}
