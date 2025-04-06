package web

import (
	"fmt"
	"io"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
)

// TODO: Can this the HTML be simpler for non-flipping postcards?

//go:generate qtc -file postcard.html.qtpl

type htmlVars struct {
	types.Metadata
	ImageExt string
}

func createHTML(pc types.Postcard, format string) formats.FileWriter {
	v := htmlVars{
		Metadata: pc.Meta,
		ImageExt: "." + format,
	}
	v.Metadata.Name = pc.Name

	writer := func(w io.Writer) error {
		WriteHTML(w, v)
		return nil
	}

	return formats.NewFileWriter(fmt.Sprintf("%s.html", pc.Name), "text/html", writer)
}
