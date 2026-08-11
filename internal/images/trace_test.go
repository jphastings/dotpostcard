package images

import (
	"image"
	"testing"

	"github.com/jphastings/dotpostcard/internal/geom3d"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wobblySquareRing is a ring made of four large-area corners plus a run of
// tiny zigzag "wobble" points along the top edge, none of which contribute
// meaningfully to the enclosed area.
func wobblySquareRing() []ipoint {
	pts := []ipoint{{0, 0}}
	for x := 10; x < 100; x += 10 {
		y := 0
		if (x/10)%2 == 1 {
			y = 1
		}
		pts = append(pts, ipoint{x, y})
	}
	pts = append(pts, ipoint{100, 0}, ipoint{100, 100}, ipoint{0, 100})
	return pts
}

func alphaFromRows(rows []string) *image.Alpha {
	h, w := len(rows), len(rows[0])
	a := image.NewAlpha(image.Rect(0, 0, w, h))
	for y, row := range rows {
		for x, c := range row {
			if c == '#' {
				a.Pix[y*a.Stride+x] = 255
			}
		}
	}
	return a
}

func TestTraceSinglePixel(t *testing.T) {
	a := alphaFromRows([]string{"#"})
	pts := traceLargestContour(a, 128)
	assert.Equal(t, []ipoint{{0, 0}, {1, 0}, {1, 1}, {0, 1}}, pts)
}

func TestTraceFullFrameReachesCorners(t *testing.T) {
	a := image.NewAlpha(image.Rect(0, 0, 4, 3))
	for i := range a.Pix {
		a.Pix[i] = 255
	}
	points, err := Outline(a, false, false)
	require.NoError(t, err)
	require.Len(t, points, 4)

	xs := map[float64]bool{}
	ys := map[float64]bool{}
	for _, p := range points {
		xs[p.X] = true
		ys[p.Y] = true
	}
	assert.True(t, xs[0.0] && xs[1.0], "outline should span x from 0 to 1: %v", points)
	assert.True(t, ys[0.0] && ys[1.0], "outline should span y from 0 to 1: %v", points)
}

func TestTraceLShape(t *testing.T) {
	a := alphaFromRows([]string{
		"#.",
		"##",
	})
	pts := traceLargestContour(a, 128)
	assert.Len(t, pts, 6, "an L shape has six corners: %v", pts)
	assert.Equal(t, int64(-6), shoelace2(pts), "2×area for the 3 pixels of an L, negative as traced clockwise in y-down coords")
}

func TestTraceLargestComponentWins(t *testing.T) {
	// A dust speck above-left of the card must not become the outline
	a := alphaFromRows([]string{
		"#.....",
		"......",
		"..####",
		"..####",
	})
	pts := traceLargestContour(a, 128)
	assert.Equal(t, []ipoint{{2, 2}, {6, 2}, {6, 4}, {2, 4}}, pts)
}

func TestTraceHolesIgnored(t *testing.T) {
	a := alphaFromRows([]string{
		"#####",
		"#...#",
		"#####",
	})
	pts := traceLargestContour(a, 128)
	assert.Equal(t, []ipoint{{0, 0}, {5, 0}, {5, 3}, {0, 3}}, pts)
}

func TestSimplifyClosedRemovesNoise(t *testing.T) {
	// A square traced with a single-pixel nick on one edge simplifies to the
	// square when epsilon exceeds the nick depth; the nick spans the trace's
	// wrap-around, exercising closed-loop splitting.
	a := alphaFromRows([]string{
		"####.####",
		"#########",
		"#########",
		"#########",
	})
	pts := traceLargestContour(a, 128)
	simplified := simplifyClosed(pts, 1.5)
	assert.Len(t, simplified, 4, "nick should simplify away: %v", simplified)
}

func TestSimplifyToBudgetKeepsInputWhenBudgetNotExceeded(t *testing.T) {
	pts := wobblySquareRing()
	assert.Equal(t, pts, simplifyToBudget(pts, len(pts)))
	assert.Equal(t, pts, simplifyToBudget(pts, len(pts)+5))
}

func TestSimplifyToBudgetReducesToExactBudget(t *testing.T) {
	pts := wobblySquareRing()

	out := simplifyToBudget(pts, 6)
	assert.Len(t, out, 6)

	out = simplifyToBudget(pts, 1)
	assert.Len(t, out, 3, "should never simplify below 3 points")
}

func TestSimplifyToBudgetPreservesCorners(t *testing.T) {
	pts := wobblySquareRing()
	out := simplifyToBudget(pts, 4)

	corners := []ipoint{{0, 0}, {100, 0}, {100, 100}, {0, 100}}
	assert.ElementsMatch(t, corners, out, "the highest-area corners should survive over the tiny wobbles: %v", out)
}

func TestSimplifyToBudgetPreservesRingOrder(t *testing.T) {
	pts := wobblySquareRing()
	out := simplifyToBudget(pts, 6)

	idx := 0
	for _, p := range pts {
		if idx < len(out) && p == out[idx] {
			idx++
		}
	}
	assert.Equal(t, len(out), idx, "simplified points must be a subsequence of the input in ring order: %v", out)
}

// wobblyRectAlpha is a rectangle whose top edge steps up and down every
// period columns, tracing to far more points than a plain rectangle.
func wobblyRectAlpha(w, h, teeth, depth int) *image.Alpha {
	a := image.NewAlpha(image.Rect(0, 0, w, h))
	period := max(w/teeth, 2)
	for x := 0; x < w; x++ {
		top := 10
		if (x/period)%2 == 0 {
			top += depth
		}
		for y := top; y < h-10; y++ {
			a.Pix[y*a.Stride+x] = 255
		}
	}
	return a
}

func TestOutlineWithOptsRespectsMaxPointsWithoutLosingArea(t *testing.T) {
	a := wobblyRectAlpha(400, 200, 80, 4)

	full, err := Outline(a, false, false)
	require.NoError(t, err)
	require.Greater(t, len(full), 40, "fixture should trace to more points than the budget below")

	const budget = 40
	budgeted, err := OutlineWithOpts(a, false, false, OutlineOpts{MaxPoints: budget})
	require.NoError(t, err)

	assert.LessOrEqual(t, len(budgeted), budget)
	assert.InEpsilon(t, geom3d.Area(full), geom3d.Area(budgeted), 0.03, "budgeted outline should enclose nearly the same area as the full-fidelity one")
}

func TestOutlineFullyTransparent(t *testing.T) {
	a := image.NewAlpha(image.Rect(0, 0, 4, 4))
	_, err := Outline(a, false, false)
	assert.Error(t, err)
}

func TestOutlineWinding(t *testing.T) {
	a := alphaFromRows([]string{
		"###",
		"###",
	})
	points, err := Outline(a, false, false)
	require.NoError(t, err)
	assert.Negative(t, geom3d.Area(points), "outline must be wound anticlockwise")
}
