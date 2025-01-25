package html

import (
	"fmt"
	"io"
	"io/fs"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
)

// TODO: Can this the HTML be simpler for non-flipping postcards?

//go:generate qtc -file postcard.html.qtpl

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
		WriteHTML(w, v)
		return nil
	}

	return []formats.FileWriter{
		formats.NewFileWriter(fmt.Sprintf("%s.html", pc.Name), "text/html", writer),
	}, nil
}
