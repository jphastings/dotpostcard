package geom3d

import (
	"math"
)

// Assuming the points are in a slice [frontPoints..., backPoints...] lists the trios of vertex indices
// which make up the triangles of the shape. Assumes that frontPoints and backPoints are wound the _same_ way.
func SideMesh(frontPoints, backPoints []Point) []int {
	lenFront := len(frontPoints)
	lenBack := len(backPoints)
	lenBoth := lenFront + lenBack
	posFront := 0
	posBack := findClosest(frontPoints[posFront], backPoints)

	var lastTwo [2]int
	lastTwo[0] = posFront
	lastTwo[1] = lenFront + posBack

	posFront++
	posBack = (posBack + 1) % lenBack

	point := func(i int) Point {
		if i >= lenFront {
			return backPoints[i-lenFront]
		} else {
			return frontPoints[i]
		}
	}

	var triangles []int

	for i := 0; i < lenBoth; i++ {
		frontD := distance(point(lastTwo[1]), frontPoints[posFront])
		backD := distance(point(lastTwo[1]), backPoints[posBack])

		var last int
		// TODO: handle the case where the next closest point is one we've already covered (because one loop is smaller than another?)
		if frontD <= backD {
			last = posFront
			posFront = (posFront + 1) % lenFront
		} else {
			last = lenFront + posBack
			posBack = (posBack + 1) % lenBack
		}

		oldLastIsBack := lastTwo[1] >= lenFront
		newLastIsBack := last >= lenFront

		if oldLastIsBack {
			triangles = append(triangles, lastTwo[0], lastTwo[1], last)
		} else {
			triangles = append(triangles, lastTwo[1], lastTwo[0], last)
		}

		if oldLastIsBack == newLastIsBack {
			lastTwo[1] = last
		} else {
			lastTwo[0] = lastTwo[1]
			lastTwo[1] = last
		}
	}

	return triangles
}

func findClosest(target Point, list []Point) int {
	closestI := 0
	closestD := distance(target, list[closestI])
	for i, p := range list[1:] {
		// TODO: assume there is a minimum so stop after it increases
		d := distance(target, p)
		if d < closestD {
			closestI = i + 1
			closestD = d
		}
	}

	return closestI
}

func distance(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }
