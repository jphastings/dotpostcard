package css

import (
	_ "embed"
	"io"
	"io/fs"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
)

//go:embed postcards.css
var postcardCSS string

func Codec() formats.Codec { return codec{} }

type codec struct{}

func (c codec) Name() string { return "CSS" }

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	return nil, group.Files, nil
}

func (c codec) Encode(_ types.Postcard, _ *formats.EncodeOptions) ([]formats.FileWriter, error) {
	writer := func(w io.Writer) error {
		_, err := w.Write([]byte(postcardCSS))
		return err
	}

	return []formats.FileWriter{formats.NewFileWriter("postcards.css", "text/css", writer)}, nil
}
