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
	tmpl, err := template.New("postcard").Parse(postcardHTML)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse HTML template: %v", err))
	}
	htmlTmpl = tmpl
}

func Codec() formats.Codec { return codec{} }

type codec struct{}

func (c codec) Bundle(files []fs.File, _ fs.DirEntry) ([]formats.Bundle, []fs.File) {
	return nil, files
}

func (c codec) Encode(pc types.Postcard, errs chan<- error) []io.ReadCloser {
	r := formats.AsyncWriter(func(w io.WriteCloser) error {
		return htmlTmpl.Execute(w, pc)
	}, errs)

	return []io.ReadCloser{r}
}
