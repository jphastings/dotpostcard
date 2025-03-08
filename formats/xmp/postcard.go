package xmp

import (
	"github.com/jphastings/dotpostcard/types"
)

type xmpPostcard struct {
	Namespace           string              `xml:"xmlns:Postcard,attr"`
	Flip                types.Flip          `xml:"Postcard:Flip"`
	CountryCode         string              `xml:"Postcard:CountryCode,omitempty"`
	Sender              types.Person        `xml:"Postcard:Sender,omitempty"`
	Recipient           types.Person        `xml:"Postcard:Recipient,omitempty"`
	Context             langText            `xml:"Postcard:Context>rdf:Alt>rdf:li,omitempty"`
	ContextAuthor       types.Person        `xml:"Postcard:ContextAuthor,omitempty"`
	DescriptionFront    string              `xml:"Postcard:DescriptionFront,omitempty"`
	DescriptionBack     string              `xml:"Postcard:DescriptionBack,omitempty"`
	TranscriptionFront  types.AnnotatedText `xml:"Postcard:TranscriptionFront,omitempty"`
	TranscriptionBack   types.AnnotatedText `xml:"Postcard:TranscriptionBack,omitempty"`
	PhysicalThicknessMM float64             `xml:"Postcard:PhysicalThicknessMM,omitempty"`
}

func addPostcardSection(sections []interface{}, meta types.Metadata) []interface{} {
	xmp := xmpPostcard{
		Namespace:   "https://dotpostcard.org/xmp/1.0/",
		Flip:        meta.Flip,
		CountryCode: meta.Location.CountryCode,
		Sender:      meta.Sender,
		Recipient:   meta.Recipient,

		DescriptionFront:   meta.Front.Description,
		DescriptionBack:    meta.Back.Description,
		TranscriptionFront: meta.Front.Transcription,
		TranscriptionBack:  meta.Back.Transcription,

		PhysicalThicknessMM: meta.Physical.ThicknessMM,
	}

	if meta.Context.Description != "" || meta.Locale != "" {
		xmp.Context = langText{
			Text: meta.Context.Description,
			Lang: meta.Locale,
		}
		xmp.ContextAuthor = meta.Context.Author
	}

	return append(sections, xmp)
}
