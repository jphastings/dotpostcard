package types

import (
	"encoding/json"
	"time"
)

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

var _ json.Marshaler = (*Date)(nil)
var _ json.Unmarshaler = (*Date)(nil)
