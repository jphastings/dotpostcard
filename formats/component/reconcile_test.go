package component

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A card whose front photo runs to the edge and locally matches the
// backboard colour: single-sided detection indents there, but the back of
// the same card pins the front's edge back to the true card size.
func TestRemoveBorderPairFixesLowContrastIndent(t *testing.T) {
	const w, h, m = 400, 300, 30 // frame size, backboard margin
	backboard := color.NRGBA{150, 150, 155, 255}
	cardColour := color.NRGBA{235, 228, 210, 255}

	newScan := func() *image.NRGBA {
		scan := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if x >= m && x < w-m && y >= m && y < h-m {
					scan.Set(x, y, cardColour)
				} else {
					scan.Set(x, y, backboard)
				}
			}
		}
		return scan
	}

	front := newScan()
	// The front's photo occupies the right half, and for a run of rows its
	// edge region matches the backboard exactly, with a bright inner
	// boundary 20px in that border detection latches onto instead
	for y := 120; y < 170; y++ {
		for x := w - m - 20; x < w-m; x++ {
			front.Set(x, y, backboard)
		}
	}

	back := newScan()

	// At ~150px/cm the detector's own outlier tolerance (0.2cm ≈ 30px)
	// accepts the 20px indent — only the cross-side constraint can fix it
	frontOut, backOut, err := removeBorderPair(front, back, false, 150, 150)
	require.NoError(t, err)

	alphaAt := func(img image.Image, x, y int) float64 {
		_, _, _, a := img.At(x, y).RGBA()
		return float64(a) / 65535
	}

	// The indent region sits inside the card, so the pair-corrected front
	// must keep it (mostly) opaque despite the local colour match
	for _, y := range []int{130, 145, 160} {
		assert.Greaterf(t, alphaAt(frontOut, w-m-10, y), 0.5,
			"indent region at (%d,%d) should stay part of the card", w-m-10, y)
	}
	// Backboard beyond the card stays removed on both sides
	assert.Zero(t, alphaAt(frontOut, w-5, 145))
	assert.Zero(t, alphaAt(backOut, w-5, 145))
	// The clean back is unharmed by reconciliation
	assert.Equal(t, 1.0, alphaAt(backOut, w/2, h/2))
	assert.Zero(t, alphaAt(backOut, 5, 5))
}

// A profile whose median sits at (near) the full frame extent, or at (near)
// zero, means that side's own border detection collapsed (found no real
// border, or almost none) -- such a profile must not be trusted to correct
// the other side, so expectation() must yield a dim that plausible() rejects.
func TestExpectationRejectsCollapsedSource(t *testing.T) {
	const frameDim = 300

	noBorderFound := []int{298, 299, 300, 297, 299} // ~ full frame extent
	exp := expectation(noBorderFound, 1.0, 0, frameDim)
	assert.False(t, exp.plausible(frameDim), "a profile at the frame's full extent indicates collapsed detection")

	almostNothingLeft := []int{2, 3, 1, 2, 3} // ~ zero
	exp = expectation(almostNothingLeft, 1.0, 0, frameDim)
	assert.False(t, exp.plausible(frameDim), "a near-zero profile also indicates collapsed detection")

	healthy := []int{200, 205, 198, 202, 201}
	exp = expectation(healthy, 1.0, 0, frameDim)
	assert.True(t, exp.plausible(frameDim), "a genuine, non-collapsed profile must still be accepted")
}

// Two scans with no resolution metadata whose detected card dimensions give
// inconsistent scale estimates: no trustworthy pixel scale exists, so
// reconciliation must be skipped rather than force one side's dimensions
// onto the other across an unknown scale.
func TestRemoveBorderPairSkipsReconcileOnScaleMismatch(t *testing.T) {
	const w, h = 400, 300
	newScan := func(mx, my int) *image.NRGBA {
		scan := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if x >= mx && x < w-mx && y >= my && y < h-my {
					scan.Set(x, y, color.NRGBA{235, 228, 210, 255})
				} else {
					scan.Set(x, y, color.NRGBA{150, 150, 155, 255})
				}
			}
		}
		return scan
	}

	// Card 340x240 on the front but 300x240 on the back: the width-axis
	// scale estimate (340/300) disagrees with the height-axis one (240/240)
	// by far more than 5%
	front := newScan(30, 30)
	back := newScan(50, 30)

	frontOut, backOut, err := removeBorderPair(front, back, false, 0, 0)
	require.NoError(t, err)

	// With reconciliation skipped each side keeps its own detected card;
	// nothing near the edges is forcibly cut away
	assert.Equal(t, 1.0, alphaAt(frontOut, 40, h/2))
	assert.Equal(t, 1.0, alphaAt(backOut, 60, h/2))
	assert.Zero(t, alphaAt(frontOut, 5, h/2))
	assert.Zero(t, alphaAt(backOut, 5, h/2))
}
