package html

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
)

// TODO: Can this the HTML be simpler for non-flipping postcards?

//go:embed postcard.html.tmpl
var postcardHTML string
var htmlTmpl *template.Template

func init() {
	tmpl, err := template.New("postcard-html").Parse(postcardHTML)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse HTML template: %v", err))
	}
	htmlTmpl = tmpl
}

func Codec() formats.Codec { return codec{} }

func (c codec) Name() string { return "HTML" }

type codec struct{}

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	return nil, group.Files, nil
}

func (c codec) Encode(pc types.Postcard, _ formats.EncodeOptions) []formats.FileWriter {
	name := fmt.Sprintf("%s.html", pc.Name)
	writer := func(w io.Writer) error {
		pc.Meta.Name = pc.Name
		return htmlTmpl.Execute(w, pc.Meta)
	}

	return []formats.FileWriter{
		formats.NewFileWriter(name, writer),
	}
}
