package xmp

import (
	"fmt"
	"strings"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
)

type xmpIptc4xmpCoreXML struct {
	Namespace string   `xml:"xmlns:Iptc4xmpCore,attr"`
	Alt       langText `xml:"Iptc4xmpCore:AltTextAccessibility>rdf:Alt>rdf:li"`
}

func addIPTCCoreSection(sections []interface{}, meta types.Metadata) []interface{} {
	if meta.Front.Description == "" {
		return sections
	}

	text, lang := formats.AltText(meta, strings.Split(meta.Locale, "-")[0])

	return append(sections, xmpIptc4xmpCoreXML{
		Namespace: "http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/",
		Alt: langText{
			Text: fmt.Sprintf(text, meta.Front.Description, meta.Back.Transcription),
			Lang: lang,
		},
	})
}
