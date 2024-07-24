package types

import (
	"fmt"
)

func (pts Polygon) toFloats() [][]float64 {
	points := make([][]float64, len(pts.Points))
	for i, pt := range pts.Points {
		points[i] = []float64{pt.X, pt.Y}
	}
	return points
}

func (pts *Polygon) fromFloats(points [][]float64) error {
	for _, pt := range points {
		if len(pt) != 2 {
			return fmt.Errorf("%dD point given instead of 2D", len(pt))
		}

		pts.Points = append(pts.Points, Point{X: pt[0], Y: pt[1]})
	}

	return nil
}
