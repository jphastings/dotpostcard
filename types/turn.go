package types

import "slices"

var clockwiseFlips = []Flip{FlipBook, FlipLeftHand, FlipCalendar, FlipRightHand}

// Turned gives the flip after turning a side clockwise by the given quarter turns (negative for
// anticlockwise). Turning both sides adds their turns together. FlipNone has no back to turn.
func (f Flip) Turned(quarters int) Flip {
	i := slices.Index(clockwiseFlips, f)
	if i < 0 {
		return f
	}
	return clockwiseFlips[((i+quarters)%4+4)%4]
}

// Turned gives the point's position (in side fractions, origin top left) after a clockwise turn of the side.
func (p Point) Turned(quarters int) Point {
	for range ((quarters % 4) + 4) % 4 {
		p = Point{X: 1 - p.Y, Y: p.X}
	}
	return p
}

// Turn updates the metadata for the front and back being turned clockwise by the given quarter turns.
func (m *Metadata) Turn(front, back int) {
	m.Flip = m.Flip.Turned(front + back)
	if front%2 != 0 {
		d := &m.Physical.FrontDimensions
		d.PxWidth, d.PxHeight = d.PxHeight, d.PxWidth
		d.CmWidth, d.CmHeight = d.CmHeight, d.CmWidth
	}
	turnSecrets(m.Front.Secrets, front)
	turnSecrets(m.Back.Secrets, back)
}

func turnSecrets(secrets []Polygon, quarters int) {
	for _, s := range secrets {
		for i, p := range s.Points {
			s.Points[i] = p.Turned(quarters)
		}
	}
}
