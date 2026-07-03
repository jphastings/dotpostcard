package component

import (
	"errors"
	"image"
	"image/color"
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

type rollingColor struct {
	av     float64
	av2    float64
	stdDev float64
}

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
			return nil, err
		}
		edges[side] = edge
	}

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

func borderFinder(img *image.Gray, fromRow, rows int) func(color.Color) bool {
	bounds := img.Bounds()
	var n uint32
	var stats rollingColor

	addToStats := func(c color.Color) {
		n++

		gray, _, _, _ := c.RGBA()
		stats.av = stats.av + (float64(gray)-float64(stats.av))/float64(n)
		stats.av2 = stats.av2 + (float64(gray)*float64(gray)-stats.av2)/float64(n)
	}

	for y := fromRow; y < fromRow+rows; y++ {
		for x := 0; x < bounds.Dx(); x++ {
			addToStats(img.At(x, y))
		}
	}

	stats.stdDev = math.Sqrt(stats.av2 - (stats.av * stats.av))

	most := stats.av + 2*stats.stdDev
	thresh := most + (65535-most)*2/3

	return func(c color.Color) bool {
		gray, _, _, _ := c.RGBA()

		return float64(gray) > thresh
	}
}

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

	bImg := absVerticalSobel(img)

	isEdge := borderFinder(bImg, void+2, borderMinThick)

	candidates := make([]int, w)
	modeTrack := make(map[int]int)
	modeMax, modeY := 0, 0
	for x := 0; x < w; x++ {
		candidates[x] = -1
		// +2 keeps the Sobel response of the fill boundary itself out of view
		for y := void + 2; y < searchDepth; y++ {
			if isEdge(bImg.At(x, y)) {
				// Torn edges fade too gradually for Sobel, so its response
				// can be well inside the fibre zone; walk back out through
				// anything brighter than the backboard.
				ref := columnBackboardRef(img, x, void)
				candidates[x] = walkOutThroughFibre(img, x, y, ref, void)
				modeTrack[candidates[x]]++
				if modeTrack[candidates[x]] > modeMax {
					modeMax, modeY = modeTrack[candidates[x]], candidates[x]
				}
				break
			}
		}
	}

	type accepted struct{ x, y int }
	var pts []accepted
	for x, y := range candidates {
		if y >= 0 && absInt(y-modeY) <= maxDeviation {
			pts = append(pts, accepted{x, y})
		}
	}

	edge := make([]int, w)
	if len(pts) == 0 {
		// No detectable border on this side: cut nothing
		return edge, nil
	}

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

// voidRows counts synthetic fill rows at the outer edge: rows whose values
// are near-constant across the full width, which scanner noise never
// produces. At most the outermost eighth is considered.
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
	}
	return bounds.Dy() / 8
}

// columnBackboardRef samples the backboard brightness at the outer end of
// the column, past any synthetic fill.
func columnBackboardRef(img *image.Gray, x, void int) float64 {
	var ref float64
	for y := void; y < void+borderMinThick; y++ {
		ref += float64(img.GrayAt(x, y).Y)
	}
	return ref / borderMinThick
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

// absVerticalSobel is the magnitude of the vertical Sobel gradient |Gy|,
// responding to horizontal edges of either polarity — the border must be
// found whether the card is lighter or darker than the backboard.
func absVerticalSobel(img *image.Gray) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))

	clampX := func(x int) int { return min(w-1, max(0, x)) }
	for y := 1; y < h-1; y++ {
		for x := 0; x < w; x++ {
			x0, x1 := clampX(x-1), clampX(x+1)
			above := int(img.GrayAt(x0, y-1).Y) + 2*int(img.GrayAt(x, y-1).Y) + int(img.GrayAt(x1, y-1).Y)
			below := int(img.GrayAt(x0, y+1).Y) + 2*int(img.GrayAt(x, y+1).Y) + int(img.GrayAt(x1, y+1).Y)
			out.SetGray(x, y, color.Gray{Y: uint8(min(255, absInt(below-above)))})
		}
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
