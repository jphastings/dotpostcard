package images

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"sort"

	rdp "github.com/calvinfeng/rdp-path-simplification"
	"github.com/jphastings/dotpostcard/internal/geom3d"
)

// Returns the outline of the image's transparency as an _anticlockwise_ series of X/Y points
func Outline(im image.Image, invertX, invertY bool) ([]geom3d.Point, error) {
	b := im.Bounds()

	// Find a starting black pixel
	var start image.Point
	found := false
	for y := b.Min.Y; y < b.Max.Y && !found; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if isOpaque(im.At(x, y)) {
				start = image.Pt(x, y)
				found = true
				break
			}
		}
	}

	// There's no transparency here
	if !found {
		return []geom3d.Point{
			{X: 0, Y: 1},
			{X: 0, Y: 0},
			{X: 1, Y: 0},
			{X: 1, Y: 1},
		}, nil
	}

	var outline []rdp.Point

	// Contour tracing using Moore Neighbour Tracing
	pos := start
	dir := 0 // Initial direction (left)
	for {
		outline = append(outline, rdp.Point{
			X: float64(pos.X+b.Min.X) / float64(b.Max.X-b.Min.X),
			Y: float64(pos.Y+b.Min.Y) / float64(b.Max.Y-b.Min.Y),
		})
		nextPos, nextDir := nextEdgePixel(im, pos, dir)
		if nextPos == start { // Loop complete
			break
		}
		pos, dir = nextPos, nextDir
	}

	ep := 0.0022
	path := rdp.SimplifyPath(outline, ep)

	geomPath := make([]geom3d.Point, len(path))
	for i, p := range path {
		geomPath[i] = geom3d.Point{X: p.X, Y: p.Y}
		if invertX {
			geomPath[i].X = 1 - geomPath[i].X
		}
		if invertY {
			geomPath[i].Y = 1 - geomPath[i].Y
		}
	}

	if len(geomPath) < 3 {
		return nil, fmt.Errorf("the outline of the image is only %d points; this probably means there's a non-transparent spec somewhere near its top left corner", len(geomPath))
	}

	return ensureDirection(geomPath), nil
}

func ensureDirection(points []geom3d.Point) []geom3d.Point {
	n := len(points)
	area := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += (points[j].X - points[i].X) * (points[j].Y + points[i].Y)
	}

	if area > 0 {
		slices.Reverse(points)
	}

	return points
}

var directions = []image.Point{
	{-1, 0}, {-1, -1}, {0, -1}, {1, -1}, // Left, Top-Left, Up, Top-Right
	{1, 0}, {1, 1}, {0, 1}, {-1, 1}, // Right, Bottom-Right, Down, Bottom-Left
}

// nextEdgePixel finds the next edge pixel by checking neighbors in order
func nextEdgePixel(im image.Image, pos image.Point, startDir int) (image.Point, int) {
	for i := 0; i < 8; i++ { // Check all 8 directions
		dir := (startDir + i) % 8 // Rotate direction
		next := pos.Add(directions[dir])
		if isOpaque(im.At(next.X, next.Y)) {
			return next, (dir + 6) % 8 // Move to this pixel and adjust direction
		}
	}
	return pos, startDir // No movement (shouldn't happen in a valid outline)
}

func isOpaque(c color.Color) bool {
	_, _, _, a := c.RGBA()
	return a > 0x8000 // Threshold at midpoint
}

// Sort

// crossProduct calculates the cross product of vectors (p1 -> p2) and (p1 -> p3)
func crossProduct(p1, p2, p3 rdp.Point) float64 {
	return (p2.X-p1.X)*(p3.Y-p1.Y) - (p2.Y-p1.Y)*(p3.X-p1.X)
}

// distanceSquared computes the squared distance between two points (to break ties)
func distanceSquared(p1, p2 rdp.Point) float64 {
	dx, dy := p1.X-p2.X, p1.Y-p2.Y
	return dx*dx + dy*dy
}

// findLeftmostPoint finds the point with the smallest X (and Y tie-break)
func findLeftmostPoint(points []rdp.Point) int {
	minIdx := 0
	for i, p := range points {
		if p.X < points[minIdx].X || (p.X == points[minIdx].X && p.Y < points[minIdx].Y) {
			minIdx = i
		}
	}
	return minIdx
}

// sortCounterClockwise sorts points in counterclockwise order
func sortCounterClockwise(points []rdp.Point) []rdp.Point {
	if len(points) < 3 {
		return points // No need to sort if < 3 points
	}

	// Find the leftmost (or bottom-most) point as pivot
	pivotIdx := findLeftmostPoint(points)
	pivot := points[pivotIdx]

	// Sort points by polar angle relative to pivot
	sort.SliceStable(points, func(i, j int) bool {
		// Compute cross product to determine order
		cross := crossProduct(pivot, points[i], points[j])
		if cross == 0 { // Collinear points - sort by distance
			return distanceSquared(pivot, points[i]) < distanceSquared(pivot, points[j])
		}
		return cross > 0 // Counterclockwise order
	})

	return points
}
