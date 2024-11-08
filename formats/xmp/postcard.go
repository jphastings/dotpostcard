package xmp

import (
	"strings"

	"github.com/jphastings/dotpostcard/types"
)

type xmpPostcard struct {
	Namespace          string              `xml:"xmlns:Postcard,attr"`
	Flip               types.Flip          `xml:"Postcard:Flip"`
	Sender             types.Person        `xml:"Postcard:Sender,omitempty"`
	Recipient          types.Person        `xml:"Postcard:Recipient,omitempty"`
	Context            langText            `xml:"Postcard:Context,omitempty"`
	ContextAuthor      types.Person        `xml:"Postcard:ContextAuthor,omitempty"`
	DescriptionFront   string              `xml:"Postcard:DescriptionFront,omitempty"`
	DescriptionBack    string              `xml:"Postcard:DescriptionBack,omitempty"`
	TranscriptionFront types.AnnotatedText `xml:"Postcard:TranscriptionFront,omitempty"`
	TranscriptionBack  types.AnnotatedText `xml:"Postcard:TranscriptionBack,omitempty"`
}

func addPostcardSection(sections []interface{}, meta types.Metadata) []interface{} {
	xmp := xmpPostcard{
		Namespace: "https://dotpostcard.org/xmp/1.0/",
		Flip:      meta.Flip,
		Sender:    meta.Sender,
		Recipient: meta.Recipient,

		DescriptionFront:   meta.Front.Description,
		DescriptionBack:    meta.Back.Description,
		TranscriptionFront: meta.Front.Transcription,
		TranscriptionBack:  meta.Back.Transcription,
	}

	if meta.Context.Description != "" {
		xmp.Context = langText{
			Text: meta.Context.Description,
			Lang: strings.Split(meta.Locale, "-")[0],
		}
		xmp.ContextAuthor = meta.Context.Author
	}

	return append(sections, xmp)
}
