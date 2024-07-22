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

func (c codec) Bundle(files []fs.File, _ fs.ReadDirFS) ([]formats.Bundle, []fs.File, map[string]error) {
	return nil, files, make(map[string]error)
}

func (c codec) Encode(_ types.Postcard, _ formats.EncodeOptions, errs chan<- error) []formats.FileWriter {
	writer := func(w io.Writer) error {
		_, err := w.Write([]byte(postcardCSS))
		return err
	}

	return []formats.FileWriter{formats.NewFileWriter("postcards.css", writer, errs)}
}
