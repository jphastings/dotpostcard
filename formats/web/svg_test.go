package web

import (
	"image"
	"image/draw"
	"math"
	"testing"

	"github.com/jphastings/dotpostcard/internal/geom3d"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

func TestSVGBackClipFollowsTheRotatedBack(t *testing.T) {
	pc := testhelpers.HeterorientedLeftHand()
	pc.Back = withCutCorner(pc.Back)
	opts := images.OutlineOpts{Threshold: 64}

	for _, flip := range []types.Flip{types.FlipLeftHand, types.FlipRightHand} {
		t.Run(string(flip), func(t *testing.T) {
			rotatedBack, _ := rotateForWeb(pc.Back, flip)
			want, err := images.OutlineWithOpts(rotatedBack, false, false, opts)
			assert.NoError(t, err)
			unrotated, err := images.OutlineWithOpts(pc.Back, false, false, opts)
			assert.NoError(t, err)

			for _, p := range rotatePointsForWeb(unrotated, flip) {
				assert.Less(t, nearest(want, p), 0.01, "%v isn't on the rotated back's outline", p)
			}
		})
	}
}

func nearest(points []geom3d.Point, p geom3d.Point) float64 {
	best := math.Inf(1)
	for _, q := range points {
		best = math.Min(best, math.Hypot(p.X-q.X, p.Y-q.Y))
	}
	return best
}

// withCutCorner makes the outline asymmetric, so a wrongly rotated outline can't coincide with the right one.
func withCutCorner(img image.Image) image.Image {
	b := img.Bounds()
	cut := image.NewNRGBA(b)
	draw.Draw(cut, b, img, b.Min, draw.Src)
	draw.Draw(cut, image.Rect(b.Min.X, b.Min.Y, b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2), image.Transparent, image.Point{}, draw.Src)
	return cut
}
