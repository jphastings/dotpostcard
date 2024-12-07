package xmp

import (
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
)

type xmpIptc4xmpCoreXML struct {
	Namespace string   `xml:"xmlns:Iptc4xmpCore,attr"`
	Alt       langText `xml:"Iptc4xmpCore:AltTextAccessibility>rdf:Alt>rdf:li"`
}

func addIPTCCoreSection(sections []interface{}, meta types.Metadata) []interface{} {
	if meta.Front.Description == "" {
		return sections
	}

	text, lang := formats.AltText(meta, meta.Locale)

	return append(sections, xmpIptc4xmpCoreXML{
		Namespace: "http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/",
		Alt: langText{
			Text: text,
			Lang: lang,
		},
	})
}
