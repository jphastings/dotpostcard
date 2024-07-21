package xmp

import (
	"fmt"
	"strings"

	"github.com/jphastings/postcards/types"
)

var (
	// The first %s will contain the description of the front of the postcard, and the second the transcription of the back.
	altTextFormats = map[string]string{
		"en": "Both sides of a postcard. On the front: %s On the back, the handwritten text %s",
	}
)

type xmpIptc4xmpCoreXML struct {
	Namespace string   `xml:"xmlns:Iptc4xmpCore,attr"`
	Alt       langText `xml:"Iptc4xmpCore:AltTextAccessibility>rdf:Alt>rdf:li"`
}

func addIPTCCoreSection(sections []interface{}, meta types.Metadata) []interface{} {
	if meta.Front.Description == "" {
		return sections
	}

	lang := strings.Split(meta.Locale, "-")[0]
	text, ok := altTextFormats[lang]
	if !ok {
		lang = "en"
		text = altTextFormats["en"]
	}

	return append(sections, xmpIptc4xmpCoreXML{
		Namespace: "http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/",
		Alt: langText{
			Text: fmt.Sprintf(text, meta.Front.Description, meta.Back.Transcription),
			Lang: lang,
		},
	})
}
