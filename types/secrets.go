package types

import (
	"fmt"
)

type SecretType struct {
	Type string `json:"type" yaml:"type"`
}

type SecretPolygon struct {
	Type      string  `json:"type" yaml:"type"`
	Prehidden bool    `json:"prehidden" yaml:"prehidden"`
	Points    []Point `json:"points" yaml:"points"`
}

type SecretBox struct {
	Type      string  `json:"type" yaml:"type"`
	Prehidden bool    `json:"prehidden" yaml:"prehidden"`
	Width     float64 `json:"width" yaml:"width"`
	Height    float64 `json:"height" yaml:"height"`
	Left      float64 `json:"left" yaml:"left"`
	Top       float64 `json:"top" yaml:"top"`
}

// Allows polygons with type: box *and* type: polygon to be unmarshalled
func (poly *Polygon) multiPolygonUnmarshaller(decode func(interface{}) error) error {
	var typer SecretType
	if err := decode(&typer); err != nil {
		return fmt.Errorf("invalid secret definition")
	}

	switch typer.Type {
	case "box":
		var box SecretBox
		if err := decode(&box); err != nil {
			return fmt.Errorf("invalid box secret definition")
		}

		return box.intoPolygon(poly)
	case "polygon":
		var polygon SecretPolygon
		if err := decode(&polygon); err != nil {
			return fmt.Errorf("invalid polygon secret definition")
		}

		poly.Prehidden = polygon.Prehidden
		poly.Points = polygon.Points

		return nil
	default:
		return fmt.Errorf("unknown secret type: %s", typer.Type)
	}
}

func (box SecretBox) intoPolygon(poly *Polygon) error {
	poly.Prehidden = box.Prehidden

	if outOfBounds(box.Width) {
		return fmt.Errorf("width of box secret is larger than 100%% of the postcard")
	}
	if outOfBounds(box.Height) {
		return fmt.Errorf("height of box secret is larger than 100%% of the postcard")
	}
	if outOfBounds(box.Left) {
		return fmt.Errorf("left edge of box secret is outside the postcard")
	}
	if outOfBounds(box.Top) {
		return fmt.Errorf("top edge of box secret is outside the postcard")
	}

	bottom := box.Top + box.Height
	if outOfBounds(bottom) {
		return fmt.Errorf("bottom edge of box secret is outside the postcard")
	}
	right := box.Left + box.Width
	if outOfBounds(right) {
		return fmt.Errorf("right edge of box secret is outside the postcard")
	}

	poly.Points = []Point{
		{X: box.Left, Y: box.Top},
		{X: right, Y: box.Top},
		{X: right, Y: bottom},
		{X: box.Left, Y: bottom},
	}

	return nil
}

func outOfBounds(d float64) bool {
	return d < 0.0 || d > 1.0
}
