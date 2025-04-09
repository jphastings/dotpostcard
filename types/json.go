package types

import (
	"encoding/json"
	"fmt"
	"time"
)

func (poly *Polygon) UnmarshalJSON(b []byte) error {
	return poly.multiPolygonUnmarshaller(func(into interface{}) error {
		return json.Unmarshal(b, into)
	})
}

func (d *Date) UnmarshalJSON(b []byte) (err error) {
	str := string(b)
	if str == "null" {
		d.Time = time.Time{}
		return nil
	}

	d.Time, err = time.Parse(`"2006-01-02"`, str)
	return err
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(d.Time.Format(`"2006-01-02"`)), nil
}

func (c Color) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"#%02X%02X%02X"`, c.R, c.G, c.B)), nil
}

func (c *Color) UnmarshalJSON(bb []byte) error {
	r, g, b, err := rgbFromString(string(bb))
	if err != nil {
		return err
	}

	c.R, c.G, c.B = r, g, b
	return nil
}

var _ json.Marshaler = (*Date)(nil)
var _ json.Unmarshaler = (*Date)(nil)
var _ json.Unmarshaler = (*Polygon)(nil)
var _ json.Marshaler = (*Color)(nil)
var _ json.Unmarshaler = (*Color)(nil)
