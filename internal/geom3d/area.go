package geom3d

// Calcualtes the area within the closed loop set of points provided, positive means clockwise wound, negative means anticlockwise
func Area(points []Point) float64 {
	n := len(points)
	area := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += (points[j].X - points[i].X) * (points[j].Y + points[i].Y)
	}
	return area
}
