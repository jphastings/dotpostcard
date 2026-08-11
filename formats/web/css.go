package web

import (
	_ "embed"
	"io"

	"github.com/jphastings/dotpostcard/formats"
)

//go:embed postcards.css
var PostcardCSS string

func createCSS() formats.FileWriter {
	writer := func(w io.Writer) error {
		_, err := w.Write([]byte(PostcardCSS))
		return err
	}

	return formats.NewSharedFileWriter("postcards.css", "text/css", writer)
}
