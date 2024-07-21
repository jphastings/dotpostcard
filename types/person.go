package types

import (
	"encoding/xml"
	"fmt"
)

type Person struct {
	Name string `json:"name"`
	Uri  string `json:"uri,omitempty" yaml:"link,omitempty"`
}

func (p Person) String() string {
	if p.Uri == "" {
		return p.Name
	}

	return fmt.Sprintf("%s (%s)", p.Name, p.Uri)
}

func (p Person) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(p.String(), start)
}
