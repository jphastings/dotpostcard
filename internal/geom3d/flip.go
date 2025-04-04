package geom3d

import "github.com/jphastings/dotpostcard/types"

func RotateForFlip(points []Point, flip types.Flip) []Point {
	var sinAngle float64
	var cosAngle float64

	switch flip {
	case types.FlipRightHand:
		sinAngle = 1
		cosAngle = 0
	case types.FlipCalendar:
		sinAngle = 0
		cosAngle = -1
	case types.FlipLeftHand:
		sinAngle = -1
		cosAngle = 0

	default: // None and Book
		return points
	}

	newPoints := make([]Point, len(points))
	for i, p := range points {
		x := p.X - 0.5
		y := p.Y - 0.5
		newPoints[i] = Point{
			X: x*cosAngle - y*sinAngle + 0.5,
			Y: x*sinAngle - y*cosAngle + 0.5,
		}
	}

	return newPoints
}
