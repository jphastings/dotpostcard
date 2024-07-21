package css

import (
	_ "embed"
	"io"
	"io/fs"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
)

//go:embed postcards.css
var postcardCSS string

func Codec() formats.Codec { return codec{} }

type codec struct{}

func (c codec) Bundle(files []fs.File, _ fs.DirEntry) ([]formats.Bundle, []fs.File) {
	return nil, files
}

func (c codec) Encode(_ types.Postcard, errs chan<- error) []io.ReadCloser {
	r := formats.AsyncWriter(func(w io.WriteCloser) error {
		_, err := w.Write([]byte(postcardCSS))
		return err
	}, errs)

	return []io.ReadCloser{r}
}
