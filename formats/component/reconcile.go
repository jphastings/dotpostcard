package component

import (
	"math"
	"sort"
)

// The two sides of a postcard are the same physical object, so their
// detected card dimensions must agree. Each side's per-row width (and
// per-column height) profile is compared against the other side's typical
// dimension; where it disagrees beyond what the other side's own variation
// allows, the offending edge — the one further from its side's median line —
// is pinned back to the expected dimension. Die-cut cards are protected by
// deriving the tolerance from the other side's profile variation: a zigzag
// edge widens its own tolerance.

// Edge array indexing (see matteWithEdges): edges[0]=top by x, edges[1]=left
// by rotated column h-1-y, edges[2]=bottom by w-1-x, edges[3]=right by y.

func leftAt(edges [4][]int, h, y int) int      { return edges[1][h-1-y] }
func rightAt(edges [4][]int, w, y int) int     { return w - 1 - edges[3][y] }
func topAt(edges [4][]int, x int) int          { return edges[0][x] }
func bottomAt(edges [4][]int, w, h, x int) int { return h - 1 - edges[2][w-1-x] }

func widthProfile(edges [4][]int, w, h int) []int {
	p := make([]int, h)
	for y := 0; y < h; y++ {
		p[y] = rightAt(edges, w, y) - leftAt(edges, h, y)
	}
	return p
}

func heightProfile(edges [4][]int, w, h int) []int {
	p := make([]int, w)
	for x := 0; x < w; x++ {
		p[x] = bottomAt(edges, w, h, x) - topAt(edges, x)
	}
	return p
}

type expectedDim struct {
	dim int // the other side's typical card dimension, in this side's pixels
	tol int // allowed deviation before a row/column is corrected
}

// expectation summarises the other side's dimension profile. scale converts
// the other side's pixels into this side's; pxPerCm is this side's
// resolution (0 when unknown). sourceFrameDim is the frame dimension, in the
// SOURCE image's own pixels, that otherProfile's values are bounded by. If
// the profile's median sits at (near) that full extent or at (near) zero,
// the source side's own border detection collapsed — found no real border,
// or almost none — so otherProfile can't be trusted; an expectedDim that
// plausible() rejects is returned instead of a bogus expectation.
func expectation(otherProfile []int, scale, pxPerCm float64, sourceFrameDim int) expectedDim {
	med := median(otherProfile)
	if !plausibleDim(med, sourceFrameDim) {
		return expectedDim{}
	}
	spread := percentile(otherProfile, 90) - percentile(otherProfile, 10)

	base := 8
	if px := int(math.Round(0.05 * pxPerCm)); px > base {
		base = px
	}
	return expectedDim{
		dim: int(math.Round(float64(med) * scale)),
		tol: base + int(math.Round(float64(spread)*scale)),
	}
}

// plausible rejects expectations that indicate the other side's detection
// failed outright (no border found ⇒ near-full-frame, or collapsed).
func (e expectedDim) plausible(frameDim int) bool {
	return e.dim > frameDim/5 && e.dim < frameDim*49/50
}

// plausibleDim reports whether a profile median looks like a genuine card
// dimension rather than a collapsed detection (near-full-frame or near-zero).
func plausibleDim(med, frameDim int) bool {
	return med > frameDim/5 && med < frameDim*24/25
}

// estimateScale infers the front-pixels-per-back-pixel scale from the two
// sides' detected card dimensions, for scan pairs without resolution
// metadata. Each axis gives an independent estimate; a scale is only
// trustworthy when both derive from plausible (non-collapsed) detections
// and agree within 5% — then their geometric mean is returned.
func estimateScale(fWidths, fHeights, bWidths, bHeights []int, fw, fh, bWidthDim, bHeightDim int) (float64, bool) {
	fW, fH := median(fWidths), median(fHeights)
	bW, bH := median(bWidths), median(bHeights)
	if !plausibleDim(fW, fw) || !plausibleDim(bW, bWidthDim) ||
		!plausibleDim(fH, fh) || !plausibleDim(bH, bHeightDim) {
		return 0, false
	}
	widthScale := float64(fW) / float64(bW)
	heightScale := float64(fH) / float64(bH)
	if widthScale > heightScale*1.05 || heightScale > widthScale*1.05 {
		return 0, false
	}
	return math.Sqrt(widthScale * heightScale), true
}

func reconcileWidths(edges *[4][]int, w, h int, exp expectedDim) {
	if !exp.plausible(w) {
		return
	}

	medLeft := median(edges[1])
	medRight := median(edges[3])
	for y := 0; y < h; y++ {
		l, r := leftAt(*edges, h, y), rightAt(*edges, w, y)
		if absInt(r-l-exp.dim) <= exp.tol {
			continue
		}
		if absInt(edges[1][h-1-y]-medLeft) >= absInt(edges[3][y]-medRight) {
			edges[1][h-1-y] = clampInt(r-exp.dim, 0, w-1)
		} else {
			edges[3][y] = clampInt(w-1-(l+exp.dim), 0, w-1)
		}
	}
}

func reconcileHeights(edges *[4][]int, w, h int, exp expectedDim) {
	if !exp.plausible(h) {
		return
	}

	medTop := median(edges[0])
	medBottom := median(edges[2])
	for x := 0; x < w; x++ {
		t, b := topAt(*edges, x), bottomAt(*edges, w, h, x)
		if absInt(b-t-exp.dim) <= exp.tol {
			continue
		}
		if absInt(edges[0][x]-medTop) >= absInt(edges[2][w-1-x]-medBottom) {
			edges[0][x] = clampInt(b-exp.dim, 0, h-1)
		} else {
			edges[2][w-1-x] = clampInt(h-1-(t+exp.dim), 0, h-1)
		}
	}
}

func median(vals []int) int {
	return percentile(vals, 50)
}

func percentile(vals []int, p int) int {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]int, len(vals))
	copy(sorted, vals)
	sort.Ints(sorted)
	return sorted[min(len(sorted)-1, len(sorted)*p/100)]
}

func clampInt(v, lo, hi int) int {
	return min(hi, max(lo, v))
}
