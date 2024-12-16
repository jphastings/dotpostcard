package types

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"sort"
	"strings"
)

type AnnotatedText struct {
	Text        string       `json:"text,omitempty" yaml:"text,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

type Annotation struct {
	Type  AnnotationType `json:"type"`
	Value string         `json:"value,omitempty"`
	// The *byte* count just before this annotation starts
	Start uint `json:"start"`
	// The *byte* count just after this annotation ends
	End uint `json:"end"`
}

type AnnotationType string

const (
	ATLocale    AnnotationType = "locale"
	ATEmphasis  AnnotationType = "em"
	ATStrong    AnnotationType = "strong"
	ATUnderline AnnotationType = "underline"
)

func (at AnnotatedText) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	enc, err := json.Marshal(at)
	if err != nil {
		return err
	}
	return e.EncodeElement(string(enc), start)
}

type token struct {
	isOpen bool
	pos    uint
	Annotation
}

type byPos []token

func (a byPos) Len() int      { return len(a) }
func (a byPos) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byPos) Less(i, j int) bool {
	return a[i].pos < a[j].pos
}

func (at AnnotatedText) HTML() string {
	var tokens []token
	for _, a := range at.Annotations {
		tokens = append(tokens, token{
			isOpen:     true,
			pos:        a.Start,
			Annotation: a,
		}, token{
			isOpen:     false,
			pos:        a.End,
			Annotation: a,
		})
	}

	sort.Sort(byPos(tokens))

	pos := uint(0)
	var outHTML strings.Builder
	for _, tok := range tokens {
		if tok.pos > pos {
			outHTML.WriteString(html.EscapeString(at.Text[pos:tok.pos]))
			pos = tok.pos
		}
		outHTML.WriteString(tok.Annotation.HTMLTag(tok.isOpen))
	}
	if pos < uint(len(at.Text)) {
		outHTML.WriteString(html.EscapeString(at.Text[pos:]))
	}

	return outHTML.String()
}

var htmlMap = map[AnnotationType]string{
	ATEmphasis:  "em",
	ATStrong:    "strong",
	ATUnderline: "underline",
}

func (a Annotation) HTMLTag(isOpen bool) string {
	switch a.Type {
	case ATLocale:
		if isOpen {
			return fmt.Sprintf(`<span lang="%s">`, a.Value)
		} else {
			return "</span>"
		}
	default:
		tag, ok := htmlMap[a.Type]
		if !ok {
			return ""
		}
		if isOpen {
			return fmt.Sprintf("<%s>", tag)
		} else {
			return fmt.Sprintf("</%s>", tag)
		}
	}
}
