package images

import (
	"image"
	"testing"

	"github.com/jphastings/dotpostcard/internal/geom3d"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
