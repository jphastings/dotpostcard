package appcore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/component"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/types"
)

// CompiledCard is the result of CompilePostcard: an encoded web-format
// postcard file ready for AddCardToCollection or writing to disk.
type CompiledCard struct {
	filename string
	mimetype string
	data     []byte
}

// Filename returns the compiled file's name (eg. "some-card.postcard.webp").
func (c *CompiledCard) Filename() string {
	return c.filename
}

// Mimetype returns the compiled file's image mimetype (eg. "image/webp").
func (c *CompiledCard) Mimetype() string {
	return c.mimetype
}

// Data returns the compiled file's encoded bytes.
func (c *CompiledCard) Data() []byte {
	return c.data
}

// CompilePostcard builds a web-format postcard file from raw front (and,
// optionally, back) image bytes plus metadata JSON (in the shape of
// types.Metadata; its "name" field is ignored on the way in, since
// types.Metadata.Name is deliberately excluded from JSON — name is given as
// its own parameter instead). It performs the same resolution detection,
// forced-size application, flip/orientation validation, border removal and
// secret hiding as the CLI's compile command, then encodes the result with
// formats/web.DefaultCodec.
//
// removeBorder attempts to convert a uniformly coloured scan background to
// transparency; archival produces a lossless (larger) output file rather
// than one sized/compressed for everyday viewing. Errors returned here wrap
// the underlying decode/validate failure with brief context, but keep its
// message intact — it's meant to surface directly in the app's error
// alerts.
func CompilePostcard(name, metaJSON string, front, back []byte, removeBorder, archival bool) (*CompiledCard, error) {
	var meta types.Metadata
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return nil, fmt.Errorf("parsing postcard metadata: %w", err)
	}
	meta.Name = name

	frontRC := io.NopCloser(bytes.NewReader(front))
	var backRCs []io.ReadCloser
	if len(back) > 0 {
		backRCs = append(backRCs, io.NopCloser(bytes.NewReader(back)))
	}

	bundle := component.BundleFromReaders(meta, frontRC, backRCs...)

	pc, err := bundle.Decode(formats.DecodeOptions{RemoveBorder: removeBorder})
	if err != nil {
		return nil, fmt.Errorf("compiling postcard: %w", err)
	}

	fws, err := web.DefaultCodec.Encode(pc, &formats.EncodeOptions{Archival: archival})
	if err != nil {
		return nil, fmt.Errorf("encoding postcard: %w", err)
	}
	if len(fws) == 0 {
		return nil, fmt.Errorf("encoding postcard: codec produced no output file")
	}

	data, err := fws[0].Bytes()
	if err != nil {
		return nil, fmt.Errorf("encoding postcard: %w", err)
	}

	return &CompiledCard{
		filename: fws[0].Filename,
		mimetype: fws[0].Mimetype,
		data:     data,
	}, nil
}
