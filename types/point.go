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
