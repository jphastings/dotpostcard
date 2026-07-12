package component

import (
	"image"
	"image/color"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func alphaAt(img image.Image, x, y int) float64 {
	_, _, _, a := img.At(x, y).RGBA()
	return float64(a) / 65535
}

func TestRemoveBorderSyntheticRoundedCard(t *testing.T) {
	// A rounded-corner card on a slightly shaded backboard
	const w, h, m, r = 400, 300, 40, 30 // margin, corner radius
	scan := image.NewNRGBA(image.Rect(0, 0, w, h))
	inCard := func(x, y int) bool {
		if x < m || x >= w-m || y < m || y >= h-m {
			return false
		}
		// corner circles
		cx := min(x-m-r, 0) + max(x-(w-m-1-r), 0)
		cy := min(y-m-r, 0) + max(y-(h-m-1-r), 0)
		return cx*cx+cy*cy <= r*r
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if inCard(x, y) {
				// Bright cream: border detection is luminance-based, so the
				// card must contrast with the backboard in luminance
				scan.Set(x, y, color.NRGBA{240, 230, 200, 255})
			} else {
				shade := uint8(90 + 20*x/w)
				scan.Set(x, y, color.NRGBA{shade, shade, shade + 10, 255})
			}
		}
	}

	got, err := removeBorder(scan, 0)
	require.NoError(t, err)

	// Backboard gone
	for _, p := range []image.Point{{5, 5}, {w - 6, 5}, {5, h - 6}, {w - 6, h - 6}, {w / 2, 10}, {10, h / 2}} {
		assert.Zerof(t, alphaAt(got, p.X, p.Y), "backboard at %v should be transparent", p)
	}
	// Card interior intact
	for _, p := range []image.Point{{w / 2, h / 2}, {m + r + 5, m + r + 5}, {w - m - r - 5, h - m - r - 5}} {
		assert.Equalf(t, 1.0, alphaAt(got, p.X, p.Y), "card interior at %v should be opaque", p)
	}
	// Rounded corners preserved, not chorded or squared: just inside the
	// corner arc must be opaque-ish; just outside it must be transparent-ish
	assert.Greater(t, alphaAt(got, m+r-int(0.5*r), m+r-int(0.5*r)), 0.5, "inside the corner arc")
	assert.Less(t, alphaAt(got, m+2, m+2), 0.5, "outside the corner arc (a squared corner would cover this)")
}

// A bright cream card on a near-white backboard is exactly the low-contrast
// case the brightness-deviation detector must catch (previously invisible to
// the far-too-strict Sobel threshold). A postmark-like dark strip sits well
// inside the card, over only a minority of columns; the border scan finds
// the true (shallow) card edge before ever reaching the (deeper) ink, so it
// must not distort the detected cut.
func TestRemoveBorderLowContrastWithPostmark(t *testing.T) {
	const w, h, m = 400, 300, 30
	// 245, not 250: voidRows treats a uniform region >=250 as synthetic
	// fill rather than real backboard, which would sample the wrong rows
	// for the backboard reference.
	backboard := color.NRGBA{245, 245, 245, 255}
	card := color.NRGBA{205, 205, 205, 255}
	ink := color.NRGBA{80, 80, 80, 255}

	scan := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x >= m && x < w-m && y >= m && y < h-m {
				scan.Set(x, y, card)
			} else {
				scan.Set(x, y, backboard)
			}
		}
	}
	// Postmark-like ink, well inside the card (not near any edge) and over a
	// minority of columns only.
	for y := m + 15; y < m+19; y++ {
		for x := m + 60; x < m+120; x++ {
			scan.Set(x, y, ink)
		}
	}

	got, err := removeBorder(scan, 0)
	require.NoError(t, err)

	// Backboard gone
	for _, p := range []image.Point{{5, 5}, {w - 6, 5}, {5, h - 6}, {w - 6, h - 6}} {
		assert.Zerof(t, alphaAt(got, p.X, p.Y), "backboard at %v should be transparent", p)
	}
	// Card interior stays opaque, including near the top edge under the
	// postmark's column range
	for _, p := range []image.Point{{w / 2, h / 2}, {m + 5, m + 5}, {m + 90, m + 5}} {
		assert.Equalf(t, 1.0, alphaAt(got, p.X, p.Y), "card interior at %v should be opaque", p)
	}
}

// A card scanned flush against one edge has no backboard there — only the
// synthetic fill added by crop/deskew tools. On that side the detector's
// reference band is the card itself, so ink inside the card is the first
// thing that deviates from it; the flush-side check must cut just the fill
// instead of chasing the ink, while the other three sides still find their
// real backboard border.
func TestRemoveBorderFlushTopEdge(t *testing.T) {
	const w, h, m, fill = 400, 300, 30, 10
	backboard := color.NRGBA{120, 120, 120, 255}
	card := color.NRGBA{230, 225, 205, 255}
	fillWhite := color.NRGBA{250, 250, 250, 255}
	ink := color.NRGBA{60, 60, 60, 255}

	scan := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case y < fill:
				scan.Set(x, y, fillWhite)
			case x >= m && x < w-m && y < h-m: // card flush to the fill at top
				scan.Set(x, y, card)
			default:
				scan.Set(x, y, backboard)
			}
		}
	}
	// Postmark-like ink band inside the card: without the flush-side check
	// this is the first deviation from the (card-coloured) reference and
	// would be mistaken for the border
	for y := 40; y < 55; y++ {
		for x := m + 20; x < w-m-20; x++ {
			scan.Set(x, y, ink)
		}
	}

	got, err := removeBorder(scan, 0)
	require.NoError(t, err)

	// Card content between the fill boundary and the ink stays opaque
	for _, p := range []image.Point{{w / 2, fill + 15}, {m + 30, fill + 20}} {
		assert.Equalf(t, 1.0, alphaAt(got, p.X, p.Y), "card at %v should stay opaque", p)
	}
	// The other three sides still cut their real backboard
	assert.Zero(t, alphaAt(got, 5, h/2))
	assert.Zero(t, alphaAt(got, w-5, h/2))
	assert.Zero(t, alphaAt(got, w/2, h-5))
	assert.Equal(t, 1.0, alphaAt(got, w/2, h/2))
}

func TestRemoveBorderSeattleZigzag(t *testing.T) {
	scan := testhelpers.TestImages["removeborder-seattle-scan.jpeg"]
	require.NotNil(t, scan)
	truthImg := testhelpers.TestImages["removeborder-seattle-alpha.png"]
	require.NotNil(t, truthImg)

	// 325.12dpi scan at original scale (synthetic crop-fill borders removed)
	pxPerCm := 325.12 / 2.54

	got, err := removeBorder(scan, pxPerCm)
	require.NoError(t, err)

	b := truthImg.Bounds()
	w, h := b.Dx(), b.Dy()
	var misclass, n int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tG, _, _, _ := truthImg.At(b.Min.X+x, b.Min.Y+y).RGBA()
			tOpaque := tG >= 0x8000
			gOpaque := alphaAt(got, x, y) >= 0.5
			if tOpaque != gOpaque {
				misclass++
			}
			n++
		}
	}

	// The die-cut zigzag border must survive detection + matting: only a
	// thin ribbon along the edge may disagree with the hand-made mask.
	assert.Less(t, float64(misclass)/float64(n), 0.01,
		"shape disagreement with hand mask: %d of %d px", misclass, n)
}
