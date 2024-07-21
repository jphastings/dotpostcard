package formats

import (
	"io"
	"io/fs"

	"github.com/jphastings/postcards/types"
)

type EncodeOptions struct {
	Archival bool
}

// Codec structs hold mechanisms for storing and reading postcard information in a specific format
type Codec interface {
	// Bundle must extract any single/set of postcard file(s) that can be decoded by this codec
	// from the given input files (which will all be in the same directory), including any directly associated
	Bundle([]fs.File, fs.ReadDirFS) ([]Bundle, []fs.File, map[string]error)

	// Encode must produce any files needed to represent postcards in this format.
	Encode(types.Postcard, EncodeOptions, chan<- error) []FileWriter
}

type FileWriter struct {
	io.ReadCloser
	filename string
}

// NewFileWriter is a helper function for creating a read stream for the return values of Encoders
func NewFileWriter(filename string, fn func(w io.WriteCloser) error, errs chan<- error) FileWriter {
	r, w := io.Pipe()
	go func(fn func(w io.WriteCloser) error, w io.WriteCloser, errs chan<- error) {
		if err := fn(w); err != nil {
			errs <- err
		}
	}(fn, w, errs)

	return FileWriter{
		filename:   filename,
		ReadCloser: r,
	}
}
