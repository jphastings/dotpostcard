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
