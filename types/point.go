package types

import (
	"encoding/json"
	"fmt"
)

// Point represents a point on a postcard, stored as a percentage of the width (X) or height (Y)
// where 1.0 is the right, or bottom, of the postcard, and 0 is the left, or top.
type Point struct {
	X float64
	Y float64
}

func (p Point) ToPixels(w, h int) (float64, float64) {
	return p.X * float64(w), p.Y * float64(h)
}

func (p Point) MarshalJSON() ([]byte, error) {
	return json.Marshal([]float64{p.X, p.Y})
}

func (p *Point) UnmarshalJSON(b []byte) error {
	var floats []float64
	if err := json.Unmarshal(b, &floats); err != nil {
		return err
	}
	if len(floats) != 2 {
		return fmt.Errorf("incorrect number of floats for point; wanted 2, got %d", len(floats))
	}

	p.X = floats[0]
	p.Y = floats[1]

	return nil
}

// Transforms a point (relative to a single image) so that it will point to the same spot
// (when relative to the double-sided equivalent).
func (p Point) TransformToDoubleSided(onFront bool, flip Flip) Point {
	if flip == FlipNone {
		return p
	}

	// For double sided images the front side is always correctly oriented, and half the height of the full image
	if onFront {
		return Point{
			X: p.X,
			Y: p.Y / 2,
		}
	}

	switch flip {
	case FlipLeftHand:
		return Point{
			X: p.Y,
			Y: 1 - p.X/2,
		}
	case FlipRightHand:
		return Point{
			X: (1 - p.Y),
			Y: p.X/2 + 0.5,
		}
	default: // FlipBook & FlipCalendar are the same
		return Point{
			X: p.X,
			Y: (p.Y / 2) + 0.5,
		}
	}
}

// Transforms a point (relative to a double image) so that it will point to the same spot
// (when relative to the single-sided equivalent).
func (p Point) TransformToSingleSided(onFront bool, flip Flip) Point {
	if flip == FlipNone {
		return p
	}

	if onFront {
		return Point{
			X: p.X,
			Y: p.Y * 2,
		}
	}

	switch flip {
	case FlipLeftHand:
		return Point{
			X: 2 - 2*p.Y,
			Y: p.X,
		}
	case FlipRightHand:
		return Point{
			X: 2*p.Y - 1,
			Y: (1 - p.X),
		}
	default: // FlipBook & FlipCalendar are the same
		return Point{
			X: p.X,
			Y: (p.Y * 2) - 1,
		}
	}
}
