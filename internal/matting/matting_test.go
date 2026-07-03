package matting

import (
	"image"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoxMatchesNaiveConvolution(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	w, h, r := 32, 24, 3
	src := newPlane(w, h)
	for i := range src.v {
		src.v[i] = rng.Float32()
	}

	got := box(src, r)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float64
			var n int
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					px, py := x+dx, y+dy
					if px >= 0 && px < w && py >= 0 && py < h {
						sum += float64(src.v[py*w+px])
						n++
					}
				}
			}
			assert.InDelta(t, sum/float64(n), got.v[y*w+x], 1e-5)
		}
	}
}

func TestGuidedFilterConstantGuidanceIsBoxBlur(t *testing.T) {
	// With featureless guidance the filter has no structure to follow, so it
	// must degrade to (approximately) a double box blur of the input.
	w, h, r := 24, 24, 4
	var I [3]*plane
	for c := range I {
		I[c] = newPlane(w, h)
		for i := range I[c].v {
			I[c].v[i] = 0.5
		}
	}
	p := newPlane(w, h)
	for y := 0; y < h; y++ {
		for x := w / 2; x < w; x++ {
			p.v[y*w+x] = 1
		}
	}

	got := guidedAlpha(I, p, r, 1e-4)
	want := box(box(p, r), r)

	for i := range got.v {
		assert.InDelta(t, want.v[i], got.v[i], 0.02)
	}
}

func TestDistanceToBoundaryMonotonicity(t *testing.T) {
	w, h := 16, 16
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 8; x < w; x++ {
			mask[y*w+x] = true
		}
	}

	dist := distanceToBoundary(mask, w, h)

	// Distance grows by one chamfer step per pixel away from the x=8 edge
	for y := 0; y < h; y++ {
		assert.EqualValues(t, 0, dist[y*w+7])
		assert.EqualValues(t, 0, dist[y*w+8])
		for x := 9; x < w; x++ {
			assert.EqualValues(t, dist[y*w+x-1]+chamferScale, dist[y*w+x], "at %d,%d", x, y)
		}
	}
}

func TestBackgroundFieldRecoversShadingRamp(t *testing.T) {
	// A linear shading ramp on the backboard should be reproduced locally
	w, h, r := 48, 16, 8
	var I [3]*plane
	bgm := newPlane(w, h)
	for c := range I {
		I[c] = newPlane(w, h)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			v := 0.5 + 0.3*float32(x)/float32(w)
			for c := range I {
				I[c].v[i] = v
			}
			bgm.v[i] = 1
		}
	}

	B := backgroundField(I, bgm, r, [3]float32{0, 0, 0})
	for x := 0; x < w; x++ {
		want := 0.5 + 0.3*float32(x)/float32(w)
		assert.InDelta(t, want, B[0].v[8*w+x], 0.05, "at x=%d", x)
	}
}

func TestApplyGroundTruthComposite(t *testing.T) {
	// Composite a known soft-alpha foreground over a shaded backboard, then
	// check Apply recovers the alpha and decontaminated colour.
	const w, h = 96, 64
	rng := rand.New(rand.NewSource(7))

	// True alpha: opaque card on the right, fibrous soft edge around x=32
	trueAlpha := make([]float64, w*h)
	edge := make([]float64, h)
	for y := range edge {
		edge[y] = 32 + 3*rng.Float64()
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := float64(x) - edge[y]
			switch {
			case d > 2:
				trueAlpha[y*w+x] = 1
			case d > -2:
				trueAlpha[y*w+x] = (d + 2) / 4
			}
		}
	}

	// Card is saturated teal; backboard is a shaded grey ramp
	scan := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			a := trueAlpha[i]
			bg := 0.6 + 0.1*float64(x)/float64(w)
			r := 0.1*a + bg*(1-a)
			g := 0.6*a + bg*(1-a)
			b := 0.55*a + bg*(1-a)
			scan.Pix[i*4] = uint8(r*255 + 0.5)
			scan.Pix[i*4+1] = uint8(g*255 + 0.5)
			scan.Pix[i*4+2] = uint8(b*255 + 0.5)
			scan.Pix[i*4+3] = 255
		}
	}

	// Geometric mask: the true edge, imprecisely (as border detection gives)
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mask[y*w+x] = float64(x) > edge[y]
		}
	}

	got, err := Apply(scan, mask, Options{BandHalfWidthPx: 6})
	require.NoError(t, err)

	var alphaErr float64
	var n int
	for y := 8; y < h-8; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if float64(x) < edge[y]-8 || float64(x) > edge[y]+8 {
				continue
			}
			gotA := float64(got.Pix[i*4+3]) / 255
			alphaErr += abs(gotA - trueAlpha[i])
			n++
		}
	}
	require.Positive(t, n)
	assert.Less(t, alphaErr/float64(n), 0.12, "mean alpha error in band too high")

	// Decontamination: partially-transparent pixels should carry the card
	// colour, not the grey backboard blend
	for y := 8; y < h-8; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			a := got.Pix[i*4+3]
			if a < 64 || a > 192 {
				continue
			}
			assert.InDelta(t, 0.1*255, float64(got.Pix[i*4]), 60, "red should be decontaminated toward card colour at %d,%d (alpha %d)", x, y, a)
		}
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
