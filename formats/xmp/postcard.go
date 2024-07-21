package xmp

import (
	"strings"

	"github.com/jphastings/postcards/types"
)

type xmpPostcard struct {
	Namespace     string       `xml:"xmlns:Postcard,attr"`
	Flip          types.Flip   `xml:"Postcard:Flip"`
	Sender        types.Person `xml:"Postcard:Sender,omitempty"`
	Recipient     types.Person `xml:"Postcard:Recipient,omitempty"`
	Context       langText     `xml:"Postcard:Context,omitempty"`
	ContextAuthor types.Person `xml:"Postcard:ContextAuthor,omitempty"`
}

func addPostcardSection(sections []interface{}, meta types.Metadata) []interface{} {
	xmp := xmpPostcard{
		Namespace: "https://dotpostcard.org/xmp/1.0/",
		Flip:      meta.Flip,
		Sender:    meta.Sender,
		Recipient: meta.Recipient,
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
