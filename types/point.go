package types

// Point represents a point on a postcard, stored as a percentage of the width (X) or height (Y)
// where 1.0 is the right, or bottom, of the postcard, and 0 is the left, or top.
type Point struct {
	X float64
	Y float64
}

func (p Point) ToPixels(w, h int) (float64, float64) {
	return p.X * float64(w), p.Y * float64(h)
}
