package geom3d

import "github.com/jphastings/dotpostcard/types"

// RotateForFlip moves back-outline points (X mirrored, Y up, as images.Outline
// returns them for a back) to where the back mesh sits before the card is
// flipped. The hand flips turn the points a quarter turn about the centre (the
// back is landscape where the front is portrait, or vice versa); a calendar
// flip turns them a half turn.
func RotateForFlip(points []Point, flip types.Flip) []Point {
	var turn func(Point) Point
	switch flip {
	case types.FlipLeftHand:
		turn = func(p Point) Point { return Point{X: p.Y, Y: 1 - p.X} }
	case types.FlipRightHand:
		turn = func(p Point) Point { return Point{X: 1 - p.Y, Y: p.X} }
	case types.FlipCalendar:
		turn = func(p Point) Point { return Point{X: 1 - p.X, Y: 1 - p.Y} }
	default: // None and Book
		return points
	}

	newPoints := make([]Point, len(points))
	for i, p := range points {
		newPoints[i] = turn(p)
	}
	return newPoints
}

// BackTexCoords gives each back-outline point's position within the back's half
// of the texture (origin bottom left), which web.rotateForWeb has turned for
// the hand flips.
func BackTexCoords(points []Point, flip types.Flip) []Point {
	uvs := make([]Point, len(points))
	for i, p := range points {
		switch flip {
		case types.FlipLeftHand:
			uvs[i] = Point{X: 1 - p.Y, Y: 1 - p.X}
		case types.FlipRightHand:
			uvs[i] = Point{X: p.Y, Y: p.X}
		default:
			uvs[i] = Point{X: 1 - p.X, Y: p.Y}
		}
	}
	return uvs
}

// FlipAxis is the in-plane axis (through the card's centre) the card turns 180° about for the flip.
func FlipAxis(f types.Flip) []float64 {
	switch f {
	case types.FlipLeftHand:
		return []float64{1, 1, 0}
	case types.FlipRightHand:
		return []float64{1, -1, 0}
	case types.FlipCalendar:
		return []float64{1, 0, 0}
	default:
		return []float64{0, 1, 0}
	}
}
