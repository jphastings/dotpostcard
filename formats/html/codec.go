package html

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
)

// TODO: Can this the HTML be simpler for non-flipping postcards?

//go:embed postcard.html.tmpl
var postcardHTML string
var htmlTmpl *template.Template

func init() {
	tmpl, err := template.New("postcard-html").Funcs(template.FuncMap{
		"comment": func(msg string) template.HTML {
			return template.HTML("<!-- " + msg + " -->")
		},
	}).Parse(postcardHTML)
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

type htmlVars struct {
	types.Metadata
	ImageExt string
}

func (c codec) Encode(pc types.Postcard, _ *formats.EncodeOptions) ([]formats.FileWriter, error) {
	v := htmlVars{
		Metadata: pc.Meta,
		ImageExt: ".jpeg",
	}
	v.Metadata.Name = pc.Name

	writer := func(w io.Writer) error {
		return htmlTmpl.Execute(w, v)
	}

	return []formats.FileWriter{
		formats.NewFileWriter(fmt.Sprintf("%s.html", pc.Name), "text/html", writer),
	}, nil
}
