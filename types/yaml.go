package types

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func (f *Flip) UnmarshalYAML(y *yaml.Node) error {
	if y.ShortTag() != "!!str" {
		return fmt.Errorf("invalid flip type, expected a string")
	}

	*f = Flip(y.Value)
	return nil
}

// Go doesn't allow falling back on the default, so we have to reimplement the type here 🤦‍♂️
type fakeAnnotatedText struct {
	Text        string
	Annotations []Annotation
}

func (at *AnnotatedText) UnmarshalYAML(y *yaml.Node) error {
	if y.ShortTag() == "!!str" {
		at.Text = y.Value
		return nil
	}

	var fake fakeAnnotatedText
	if err := y.Decode(&fake); err != nil {
		return err
	}

	at.Text = fake.Text
	at.Annotations = fake.Annotations
	return nil
}

func (at AnnotatedText) MarshalYAML() (interface{}, error) {
	if at.Text == "" || len(at.Annotations) == 0 {
		return at.Text, nil
	}

	return fakeAnnotatedText{
		Text:        at.Text,
		Annotations: at.Annotations,
	}, nil
}

func (poly Polygon) MarshalYAML() (interface{}, error) {
	secret := SecretPolygon{
		Type:      "polygon",
		Prehidden: poly.Prehidden,
		Points:    poly.Points,
	}

	return secret, nil
}

func (poly *Polygon) UnmarshalYAML(y *yaml.Node) error {
	return poly.multiPolygonUnmarshaller(func(into interface{}) error {
		return y.Decode(into)
	})
}

func (p Point) MarshalYAML() (interface{}, error) {
	node := yaml.Node{
		Kind:    yaml.SequenceNode,
		Style:   yaml.FlowStyle,
		Content: make([]*yaml.Node, 2),
	}

	vals := []string{fmt.Sprintf("%f", p.X), fmt.Sprintf("%f", p.Y)}

	for i, val := range vals {
		node.Content[i] = &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: val,
		}
	}

	return &node, nil
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

func (d *Date) UnmarshalYAML(y *yaml.Node) error {
	switch y.ShortTag() {
	case "!!str":
		val := strings.TrimSuffix(strings.TrimPrefix(y.Value, `"`), `"`)
		t, err := time.Parse(`2006-01-02`, val)
		if err != nil {
			return err
		}
		d.Time = t
		return nil
	case "!!timestamp":
		t, err := time.Parse(`2006-01-02`, y.Value)
		if err != nil {
			return err
		}
		d.Time = t
		return nil
	}

	return fmt.Errorf("dates need to be in the format YYYY-MM-DD")
}

func (d Date) MarshalYAML() (interface{}, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Time.Format(`2006-01-02`), nil
}

func (c Color) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf(`"#%02X%02X%02X"`, c.R, c.G, c.B), nil
}

func (c *Color) UnmarshalYAML(y *yaml.Node) error {
	if y.ShortTag() != "!!str" {
		return fmt.Errorf("invalid color format, expected a string")
	}

	r, g, b, err := colorFromString(y.Value)
	if err != nil {
		return err
	}

	c.R, c.G, c.B = r, g, b
	return nil
}

var _ yaml.Unmarshaler = (*Flip)(nil)
var _ yaml.Marshaler = (*Polygon)(nil)
var _ yaml.Unmarshaler = (*Polygon)(nil)
var _ yaml.Marshaler = (*Size)(nil)
var _ yaml.Unmarshaler = (*Size)(nil)
var _ yaml.Marshaler = (*AnnotatedText)(nil)
var _ yaml.Unmarshaler = (*AnnotatedText)(nil)
var _ yaml.Marshaler = (*Date)(nil)
var _ yaml.Unmarshaler = (*Date)(nil)
var _ yaml.Marshaler = (*Color)(nil)
var _ yaml.Unmarshaler = (*Color)(nil)
