package geom3d

import (
	"testing"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

// Turning the card 180° about the flip's axis (as the USD tap animation does)
// brings each back vertex to a spot in the upright back; the hand flips' texture
// is stored turned a quarter (web.rotateForWeb), so that spot is read in the turned texture.
func TestFlippedBackShowsTextureUpright(t *testing.T) {
	textureTurn := map[types.Flip]func(Point) Point{
		types.FlipBook:      func(p Point) Point { return p },
		types.FlipCalendar:  func(p Point) Point { return p },
		types.FlipLeftHand:  func(p Point) Point { return Point{1 - p.Y, p.X} },
		types.FlipRightHand: func(p Point) Point { return Point{p.Y, 1 - p.X} },
	}
	outline := []Point{{0.9, 0.8}, {0.1, 0.7}, {0.2, 0.1}}

	for flip, inTexture := range textureTurn {
		t.Run(string(flip), func(t *testing.T) {
			uvs := BackTexCoords(outline, flip)
			for i, p := range RotateForFlip(outline, flip) {
				q := inTexture(halfTurnAbout(FlipAxis(flip), p))
				assert.InDeltaSlice(t, []float64{uvs[i].X, uvs[i].Y}, []float64{q.X, q.Y}, 1e-9)
			}
		})
	}
}

// halfTurnAbout reflects p's offset from the card centre across the in-plane axis, which is what a 180° turn about it does.
func halfTurnAbout(axis []float64, p Point) Point {
	vx, vy := p.X-0.5, p.Y-0.5
	k := 2 * (vx*axis[0] + vy*axis[1]) / (axis[0]*axis[0] + axis[1]*axis[1])
	return Point{k*axis[0] - vx + 0.5, k*axis[1] - vy + 0.5}
}
