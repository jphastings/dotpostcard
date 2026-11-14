package xmp

import (
	"github.com/jphastings/dotpostcard/types"
)

type xmpDC struct {
	Namespace   string    `xml:"xmlns:dc,attr"`
	Title       *langText `xml:"dc:title>rdf:Alt>rdf:li,omitempty"`
	Description langText  `xml:"dc:description>rdf:Alt>rdf:li,omitempty"`
}

var explanation = langText{
	Text: "A postcard stored in the dotpostcard format (https://dotpostcard.org)",
	Lang: "en",
}

func addDCSection(sections []interface{}, meta types.Metadata) []interface{} {
	dc := xmpDC{
		Namespace:   "http://purl.org/dc/elements/1.1/",
		Description: explanation,
	}
	if meta.Title != "" {
		dc.Title = &langText{Text: meta.Title, Lang: "x-default"}
	}
	return append(sections, dc)
}
