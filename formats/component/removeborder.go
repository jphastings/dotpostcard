package component

import (
	"errors"
	"image"
	"math"

	"github.com/jphastings/dotpostcard/internal/matting"
)

const (
	borderMinThick = 8

	// Edge points may deviate this far (physically) from a side's dominant
	// line before being treated as outliers; deep enough for die-cut zigzag
	// teeth and torn edges, which are genuine card shape, not noise.
	allowableDeviationCm    = 0.2
	minAllowableDeviationPx = 12

	// Half-width of the alpha-matting band around the detected border —
	// roughly the depth of the fibrous zone on a torn edge.
	matteBandCm    = 0.05
	minMatteBandPx = 4
	maxMatteBandPx = 40
)

var ErrAlreadyTransparent = errors.New("this image already has transparent pixels, ")

// rotation maps each side's rotated-frame coordinates onto original image
// coordinates, so every side can be scanned as if it were the top edge.
var rotation = map[int]func(image.Rectangle, int, int) (int, int){
	0: func(bnd image.Rectangle, x, y int) (int, int) { return x, y },
	1: func(bnd image.Rectangle, x, y int) (int, int) { return y, bnd.Dx() - 1 - x },
	2: func(bnd image.Rectangle, x, y int) (int, int) { return bnd.Dx() - 1 - x, bnd.Dy() - 1 - y },
	3: func(bnd image.Rectangle, x, y int) (int, int) { return bnd.Dy() - 1 - y, x },
}

// removeBorder detects the scanner backboard on all four sides of the card
// and mattes it away with soft alpha. pxPerCm may be 0 when the scan's
// resolution is unknown.
func removeBorder(img image.Image, pxPerCm float64) (image.Image, error) {
	edges, err := detectBorderEdges(img, pxPerCm)
	if err != nil {
		return nil, err
	}
	return matteWithEdges(img, edges, pxPerCm)
}

// removeBorderPair mattes both sides of the same physical card. The two
// scans must show (near-)identical card dimensions, so each side's detected
// geometry corrects the other's: where a low-contrast region (a photo close
// to the backboard colour) pulls one side's border inward, the other side's
// card size pins it back.
func removeBorderPair(front, back image.Image, heteroriented bool, pxPerCmF, pxPerCmB float64) (image.Image, image.Image, error) {
	fEdges, err := detectBorderEdges(front, pxPerCmF)
	if err != nil {
		return nil, nil, err
	}
	bEdges, err := detectBorderEdges(back, pxPerCmB)
	if err != nil {
		return nil, nil, err
	}

	fw, fh := front.Bounds().Dx(), front.Bounds().Dy()
	bw, bh := back.Bounds().Dx(), back.Bounds().Dy()

	scale := 1.0 // front pixels per back pixel
	scaleKnown := pxPerCmF > 0 && pxPerCmB > 0
	if scaleKnown {
		scale = pxPerCmF / pxPerCmB
	}

	fWidths, fHeights := widthProfile(fEdges, fw, fh), heightProfile(fEdges, fw, fh)
	bWidths, bHeights := widthProfile(bEdges, bw, bh), heightProfile(bEdges, bw, bh)
	// bWidths/bHeights are bounded by bw/bh respectively; once swapped for a
	// heteroriented back, bWidths is bounded by bh and bHeights by bw — the
	// source frame dims below must track that swap.
	bWidthSourceDim, bHeightSourceDim := bw, bh
	if heteroriented { // the back is scanned rotated 90°
		bWidths, bHeights = bHeights, bWidths
		bWidthSourceDim, bHeightSourceDim = bh, bw
	}

	reconcilable := true
	if !scaleKnown {
		// Without resolution metadata the two scans can differ in scale by
		// an unknown factor, and reconciling across a mismatched scale is
		// how one bad side mangles a good one. Estimate the scale from the
		// detected card dimensions themselves; when no trustworthy estimate
		// exists, skip reconciliation and let each side keep its own edges.
		scale, reconcilable = estimateScale(fWidths, fHeights, bWidths, bHeights,
			fw, fh, bWidthSourceDim, bHeightSourceDim)
	}

	if reconcilable {
		reconcileWidths(&fEdges, fw, fh, expectation(bWidths, scale, pxPerCmF, bWidthSourceDim))
		reconcileHeights(&fEdges, fw, fh, expectation(bHeights, scale, pxPerCmF, bHeightSourceDim))
		if heteroriented {
			reconcileWidths(&bEdges, bw, bh, expectation(fHeights, 1/scale, pxPerCmB, fh))
			reconcileHeights(&bEdges, bw, bh, expectation(fWidths, 1/scale, pxPerCmB, fw))
		} else {
			reconcileWidths(&bEdges, bw, bh, expectation(fWidths, 1/scale, pxPerCmB, fw))
			reconcileHeights(&bEdges, bw, bh, expectation(fHeights, 1/scale, pxPerCmB, fh))
		}
	}

	frontOut, err := matteWithEdges(front, fEdges, pxPerCmF)
	if err != nil {
		return nil, nil, err
	}
	backOut, err := matteWithEdges(back, bEdges, pxPerCmB)
	if err != nil {
		return nil, nil, err
	}
	return frontOut, backOut, nil
}

func detectBorderEdges(img image.Image, pxPerCm float64) ([4][]int, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	maxDeviation := int(allowableDeviationCm * pxPerCm)
	if maxDeviation < minAllowableDeviationPx {
		maxDeviation = minAllowableDeviationPx
	}
	var edges [4][]int
	for side := 0; side < 4; side++ {
		var b image.Rectangle
		if side%2 == 0 {
			b = image.Rect(0, 0, w, h)
		} else {
			b = image.Rect(0, 0, h, w)
		}
		fImg := image.NewGray(b)
		for ry := 0; ry < b.Dy(); ry++ {
			for rx := 0; rx < b.Dx(); rx++ {
				x, y := rotation[side](b, rx, ry)
				fImg.Set(rx, ry, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}

		edge, err := findBorderEdge(fImg, maxDeviation)
		if err != nil {
			return edges, err
		}
		edges[side] = edge
	}
	return edges, nil
}

func matteWithEdges(img image.Image, edges [4][]int, pxPerCm float64) (image.Image, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// The card is the intersection of the four "inside this side's border"
	// half-regions. Corners emerge from the two adjacent sides' genuine
	// edge data, so rounded or die-cut corners keep their shape.
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mask[y*w+x] = y >= edges[0][x] &&
				x >= edges[1][h-1-y] &&
				h-1-y >= edges[2][w-1-x] &&
				w-1-x >= edges[3][y]
		}
	}

	return matting.Apply(img, mask, matting.Options{
		BandHalfWidthPx: bandHalfWidth(pxPerCm, w, h),
	})
}

func bandHalfWidth(pxPerCm float64, w, h int) int {
	var band int
	if pxPerCm > 0 {
		band = int(math.Round(matteBandCm * pxPerCm))
	} else {
		band = int(math.Round(0.004 * float64(max(w, h))))
	}
	return min(maxMatteBandPx, max(minMatteBandPx, band))
}

const (
	// A column's candidate edge triggers at the first row whose brightness
	// deviates from that column's backboard reference by more than this many
	// gray levels — real card/backboard contrast, even for a cream card on a
	// white backboard, clears this by a wide margin while scanner/JPEG noise
	// does not.
	triggerDelta = 10
	// The deviation must hold for this many consecutive rows, so a single
	// noisy pixel can't trigger a false candidate.
	triggerRun = 3
)

// findBorderEdge returns, for every column of the (rotated) image, the row
// at which the scanner backboard ends and the card begins. Columns with no
// detected edge, or whose edge is an outlier relative to the side's modal
// line, are filled by interpolating between their accepted neighbours —
// never flattened to the modal line, which would erase die-cut shapes.
func findBorderEdge(img *image.Gray, maxDeviation int) ([]int, error) {
	// The border must lie within the outermost quarter of the scan; the
	// per-column scan takes the first edge from the outside, so card content
	// deeper in can't shadow the real border.
	bounds := img.Bounds()
	w, searchDepth := bounds.Dx(), bounds.Dy()/4

	// Scans are sometimes cropped/deskewed with a synthetic uniform fill
	// along the edges; real backboard rows always carry scanner noise. Skip
	// fill rows so backboard references and edge detection see real data.
	void := voidRows(img)

	// A card lying flush against the scan edge has no backboard on this
	// side: the reference band IS the card, and the first deviation from it
	// would be card content (ink, postmarks) well inside. When the outer
	// band's brightness matches the interior's, cut only the synthetic fill.
	if absFloat(bandMedian(img, void, void+borderMinThick)-bandMedian(img, searchDepth-2, searchDepth+3)) <= triggerDelta {
		return filledEdge(w, void), nil
	}

	candidates := make([]int, w)
	modeTrack := make(map[int]int)
	modeMax, modeY := 0, 0
	for x := 0; x < w; x++ {
		candidates[x] = -1
		ref := columnBackboardRef(img, x, void)
		// +2 keeps the fill boundary itself out of view
		for y := void + 2; y < searchDepth; y++ {
			triggered := true
			for k := 0; k < triggerRun; k++ {
				yk := y + k
				if yk >= bounds.Dy() || absFloat(float64(img.GrayAt(x, yk).Y)-ref) <= triggerDelta {
					triggered = false
					break
				}
			}
			if !triggered {
				continue
			}
			candidates[x] = walkOutThroughFibre(img, x, y, ref, void)
			modeTrack[candidates[x]]++
			if modeTrack[candidates[x]] > modeMax {
				modeMax, modeY = modeTrack[candidates[x]], candidates[x]
			}
			break
		}
	}

	type accepted struct{ x, y int }
	var pts []accepted
	for x, y := range candidates {
		if y >= 0 && absInt(y-modeY) <= maxDeviation {
			pts = append(pts, accepted{x, y})
		}
	}

	// A real border produces broad, consistent support across the side;
	// scattered triggers from card content (handwriting, postmarks) don't.
	// Treat no support or thin support as no border, falling back to the
	// synthetic-fill row count — fill is by definition not card.
	if len(pts) < max(1, w/10) {
		return filledEdge(w, void), nil
	}

	edge := make([]int, w)

	for x := 0; x < pts[0].x; x++ {
		edge[x] = pts[0].y
	}
	for i := 0; i < len(pts)-1; i++ {
		a, b := pts[i], pts[i+1]
		for x := a.x; x < b.x; x++ {
			edge[x] = a.y + (b.y-a.y)*(x-a.x)/(b.x-a.x)
		}
	}
	for x := pts[len(pts)-1].x; x < w; x++ {
		edge[x] = pts[len(pts)-1].y
	}

	return edge, nil
}

const (
	// A pixel this much brighter/darker than the column's backboard is
	// treated as fibre/shadow: above JPEG/scanner noise, below edge contrast.
	brightnessDelta = 10
	// This many consecutive backboard pixels end the outward fibre walk, so
	// small gaps between the outermost fibres don't stop it early.
	backboardRun = 4
)

// voidRows counts synthetic fill rows at the outer edge: crop/deskew tools
// pad with pure white or black, so a fill row is near-constant across the
// full width at an extreme value — real backboard rows are neither. At most
// the outermost eighth is considered.
func voidRows(img *image.Gray) int {
	bounds := img.Bounds()
	w := bounds.Dx()
	for y := 0; y < bounds.Dy()/8; y++ {
		lo, hi := img.GrayAt(0, y).Y, img.GrayAt(0, y).Y
		for x := 1; x < w; x++ {
			g := img.GrayAt(x, y).Y
			lo, hi = min(lo, g), max(hi, g)
			if hi-lo > 4 {
				return y
			}
		}
		if lo > 5 && hi < 250 {
			return y // uniform but not fill-coloured: real backboard
		}
	}
	return bounds.Dy() / 8
}

// bandMedian is the median brightness of every pixel in rows [from, to),
// clamped to the image.
func bandMedian(img *image.Gray, from, to int) float64 {
	bounds := img.Bounds()
	vals := make([]int, 0, (to-from)*bounds.Dx())
	for y := max(from, 0); y < min(to, bounds.Dy()); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			vals = append(vals, int(img.GrayAt(x, y).Y))
		}
	}
	return float64(median(vals))
}

func filledEdge(w, row int) []int {
	edge := make([]int, w)
	for x := range edge {
		edge[x] = row
	}
	return edge
}

// columnBackboardRef samples the backboard brightness at the outer end of
// the column, past any synthetic fill. The median of the sampled rows keeps
// the reference honest when the window is contaminated — by an antialiased
// fill-boundary pixel, or by card rows where the backboard margin is thin.
func columnBackboardRef(img *image.Gray, x, void int) float64 {
	window := make([]int, borderMinThick)
	for i := range window {
		window[i] = int(img.GrayAt(x, void+i).Y)
	}
	return float64(median(window))
}

// walkOutThroughFibre walks from a detected edge back toward the image
// border, returning the outermost row still brighter than the column's
// backboard. Torn paper edges fade too gradually for Sobel to catch, but
// their fibres are lighter than the backboard, while edge shadows are
// darker — a signed test recovers fibre without ever including shadow.
func walkOutThroughFibre(img *image.Gray, x, edgeY int, ref float64, void int) int {
	edge := edgeY
	run := 0
	for y := edgeY; y >= void; y-- {
		if float64(img.GrayAt(x, y).Y)-ref > brightnessDelta {
			run = 0
			edge = y
		} else {
			run++
			if run >= backboardRun {
				return edge
			}
		}
	}
	return edge
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
