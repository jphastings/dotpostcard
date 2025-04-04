package geom3d

import (
	"math"
)

// Triangulate lists the triangles that can be used to cover the space within the outline provided using ear clipping.
// Requires anticlockwise winding.
func Triangulate(points []Point) []int {
	n := len(points)
	if n < 3 {
		return nil // Not enough points to form a triangle
	}

	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	var triangles []int

	// Ear clipping algorithm
	for len(indices) > 3 {
		foundEar := false
		for i := 0; i < len(indices); i++ {
			prev := indices[(i-1+len(indices))%len(indices)]
			curr := indices[i]
			next := indices[(i+1)%len(indices)]

			if isEar(points, prev, curr, next, indices) {
				// Add triangle
				triangles = append(triangles, prev, curr, next)
				// Remove the ear vertex
				indices = append(indices[:i], indices[i+1:]...)
				foundEar = true
				break
			}
		}

		// If no ear was found, something went wrong (e.g., degenerate polygon)
		if !foundEar {
			break
		}
	}

	// Last triangle
	if len(indices) == 3 {
		triangles = append(triangles, indices[0], indices[1], indices[2])
	}

	return triangles
}

// isEar checks if a given vertex is an "ear" in the polygon
func isEar(points []Point, prev, curr, next int, indices []int) bool {
	// Check if the triangle (prev, curr, next) is convex
	if !isConvex(points[prev], points[curr], points[next]) {
		return false
	}

	// Check if any other point is inside this triangle
	for _, idx := range indices {
		if idx == prev || idx == curr || idx == next {
			continue
		}
		if pointInTriangle(points[idx], points[prev], points[curr], points[next]) {
			return false
		}
	}
	return true
}

// isConvex checks if three points make a convex angle
func isConvex(a, b, c Point) bool {
	return (b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X) > 0
}

// pointInTriangle checks if a point is inside the given triangle
func pointInTriangle(p, a, b, c Point) bool {
	area := func(p1, p2, p3 Point) float64 {
		return math.Abs((p1.X*(p2.Y-p3.Y) + p2.X*(p3.Y-p1.Y) + p3.X*(p1.Y-p2.Y)) / 2.0)
	}

	A := area(a, b, c)
	A1 := area(p, b, c)
	A2 := area(a, p, c)
	A3 := area(a, b, p)

	return math.Abs(A-(A1+A2+A3)) < 1e-9
}
