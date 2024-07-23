package xmp

import (
	"github.com/jphastings/postcards/types"
)

type xmpDC struct {
	Namespace   string   `xml:"xmlns:dc,attr"`
	Description langText `xml:"dc:description>rdf:Alt>rdf:li,omitempty"`
}

var explanation = langText{
	Text: "A postcard stored in the dotpostcard format (https://dotpostcard.org)",
	Lang: "en",
}

func addDCSection(sections []interface{}, _ types.Metadata) []interface{} {
	return append(sections, xmpDC{
		Namespace:   "http://purl.org/dc/elements/1.1/",
		Description: explanation,
	})
}
