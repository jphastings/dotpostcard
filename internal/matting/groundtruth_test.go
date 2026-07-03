package matting_test

import (
	"image"
	"testing"

	"github.com/jphastings/dotpostcard/internal/matting"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ground truth: crops of real scans paired with hand-made soft-alpha masks.
// The geometric input mask is the truth binarized at 50% — what an ideal
// border detector would produce — and Apply must recover the soft fibre
// detail the hand mask preserves.
func groundTruthCase(t *testing.T, scanName, truthName string, bandHalfWidth int) (*image.NRGBA, *image.Gray, []bool) {
	t.Helper()

	scan := testhelpers.TestImages[scanName]
	require.NotNil(t, scan, "missing fixture %s", scanName)
	truthImg := testhelpers.TestImages[truthName]
	require.NotNil(t, truthImg, "missing fixture %s", truthName)

	b := truthImg.Bounds()
	w, h := b.Dx(), b.Dy()
	truth := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			g, _, _, _ := truthImg.At(b.Min.X+x, b.Min.Y+y).RGBA()
			truth.Pix[y*w+x] = uint8(g >> 8)
		}
	}

	mask := make([]bool, w*h)
	for i, v := range truth.Pix {
		mask[i] = v >= 128
	}

	got, err := matting.Apply(scan, mask, matting.Options{BandHalfWidthPx: bandHalfWidth})
	require.NoError(t, err)
	return got, truth, mask
}

func assertAlphaRecovery(t *testing.T, got *image.NRGBA, truth *image.Gray) (gotSoft, truthSoft int) {
	w, h := truth.Rect.Dx(), truth.Rect.Dy()

	// "Band" here is anywhere either alpha is intermediate — the contested
	// region around the card's edge.
	var alphaErr float64
	var n, misclass int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			tA := float64(truth.Pix[i]) / 255
			gA := float64(got.Pix[i*4+3]) / 255

			if tA > 0.02 && tA < 0.98 || gA > 0.02 && gA < 0.98 {
				alphaErr += abs(gA - tA)
				n++
				if (gA >= 0.5) != (tA >= 0.5) {
					misclass++
				}
			}
			if gA > 0.05 && gA < 0.95 {
				gotSoft++
			}
			if tA > 0.05 && tA < 0.95 {
				truthSoft++
			}
		}
	}
	require.Positive(t, n)

	assert.Less(t, alphaErr/float64(n), 0.2, "mean alpha error against the hand-made mask")

	// Shape preservation: the 50%-opacity boundary must follow the hand
	// mask's — a flattened zigzag or chamfered corner blows this up far past
	// the ~1px boundary ribbon the matting legitimately shifts.
	assert.Less(t, float64(misclass)/float64(n), 0.10, "shape disagreement with the hand-made mask")

	return gotSoft, truthSoft
}

func TestApplySinCityTornCorner(t *testing.T) {
	// 600dpi scan: band half-width 0.05cm × 236px/cm ≈ 12px
	got, truth, _ := groundTruthCase(t, "matte-sin-city-scan.jpeg", "matte-sin-city-alpha.png", 12)
	gotSoft, truthSoft := assertAlphaRecovery(t, got, truth)

	// This torn corner's hand mask is genuinely fibrous, so the matte must
	// be comparably soft: neither binarised (fibres amputated) nor blurred
	// into twice as many soft pixels (fibres homogenised).
	ratio := float64(gotSoft) / float64(truthSoft)
	assert.Greater(t, ratio, 0.5, "matte is too hard: fibre detail lost")
	assert.Less(t, ratio, 2.0, "matte is too soft: fibre detail blurred away")
}

func TestApplySeattleZigzag(t *testing.T) {
	// 325dpi scan: band half-width 0.05cm × 128px/cm ≈ 6px. The die-cut
	// hand mask is near-binary, so only shape fidelity is asserted — the
	// soft-pixel ratio is meaningless against an antialiasing-only truth.
	got, truth, _ := groundTruthCase(t, "matte-seattle-scan.jpeg", "matte-seattle-alpha.png", 6)
	assertAlphaRecovery(t, got, truth)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
