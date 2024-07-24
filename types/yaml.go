package types

import (
	"fmt"
	"math/big"

	"gopkg.in/yaml.v3"
)

func (f *Flip) UnmarshalYAML(y *yaml.Node) error {
	if y.ShortTag() != "!!str" {
		return fmt.Errorf("invalid flip type, expected a string")
	}

	*f = Flip(y.Value)
	return nil
}

type SecretType struct {
	Type string `yaml:"type"`
}

type SecretPolygon struct {
	Type      string  `yaml:"type"`
	Prehidden bool    `yaml:"prehidden"`
	Points    []Point `yaml:"points"`
}

type SecretBox struct {
	Type      string  `yaml:"type"`
	Prehidden bool    `yaml:"prehidden"`
	Width     float64 `yaml:"width"`
	Height    float64 `yaml:"height"`
	Left      float64 `yaml:"left"`
	Top       float64 `yaml:"top"`
}

func (poly Polygon) MarshalYAML() (interface{}, error) {
	secret := SecretPolygon{
		Type:      "polygon",
		Prehidden: poly.Prehidden,
		Points:    poly.Points,
	}

	return secret, nil
}

func (p Point) MarshalYAML() (interface{}, error) {
	return []float64{p.X, p.Y}, nil
}

func (p *Point) UnmarshalYAML(y *yaml.Node) error {
	var floats []float64
	if err := y.Decode(&floats); err != nil {
		return err
	}
	if len(floats) != 2 {
		return fmt.Errorf("incorrect number of floats for point; wanted 2, got %d", len(floats))
	}

	p.X = floats[0]
	p.Y = floats[1]

	return nil
}

func (poly *Polygon) UnmarshalYAML(y *yaml.Node) error {
	var typer SecretType
	if err := y.Decode(&typer); err != nil {
		return fmt.Errorf("invalid secret definition")
	}

	switch typer.Type {
	case "box":
		var box SecretBox
		if err := y.Decode(&box); err != nil {
			return fmt.Errorf("invalid box secret definition")
		}

		return box.intoPolygon(poly)
	case "polygon":
		var polygon SecretPolygon
		if err := y.Decode(&polygon); err != nil {
			return fmt.Errorf("invalid polygon secret definition")
		}

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

func (s Size) MarshalYAML() (interface{}, error) {
	w, _ := s.CmWidth.Float64()
	h, _ := s.CmHeight.Float64()
	return fmt.Sprintf("%.2fcm x %.2fcm", w, h), nil
}

func (s *Size) UnmarshalYAML(y *yaml.Node) error {
	if y.ShortTag() != "!!str" {
		return fmt.Errorf("invalid front_size, expected a string")
	}

	var w, h big.Rat
	_, err := fmt.Sscanf(y.Value, "%fcm x %fcm", &w, &h)
	if err != nil {
		return err
	}

	newSize := &Size{
		CmWidth:  &w,
		CmHeight: &h,
	}
	*s = *newSize

	return err
}

var _ yaml.Marshaler = (*Polygon)(nil)
var _ yaml.Unmarshaler = (*Polygon)(nil)
var _ yaml.Marshaler = (*Size)(nil)
var _ yaml.Unmarshaler = (*Size)(nil)
