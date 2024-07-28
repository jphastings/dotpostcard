package types

import (
	"encoding/xml"
	"fmt"
	"regexp"
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

var personRE = regexp.MustCompile(`^(.+) \(([^()]+)\)$`)

func (p *Person) Scan(str string) {
	if parts := personRE.FindStringSubmatch(str); len(parts) > 0 {
		p.Name = parts[1]
		p.Uri = parts[2]
	} else {
		p.Name = str
		p.Uri = ""
	}
}

func (p Person) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(p.String(), start)
}
