package formats

import (
	"io"
	"io/fs"
	"path"
	"slices"

	"github.com/jphastings/postcards/types"
)

// Codec structs hold mechanisms for storing and reading postcard information in a specific format
type Codec interface {
	// Bundle must extract any single/set of postcard file(s) that can be decoded by this codec
	// from the given input files (which will all be in the same directory), including any directly associated
	Bundle([]fs.File, fs.DirEntry) ([]Bundle, []fs.File)

	// Encode must produce any files needed to represent postcards in this format.
	Encode(types.Postcard, chan<- error) []io.ReadCloser
}

// Bundle represents a bundle of files that will be decoded together
type Bundle interface {
	// Decode must select the first single/set of postcard file(s) in 'input'
	Decode() (pc types.Postcard, err error)
}

// HasExtensions returns true if the given file has one of the provided filename
// extensions. Provided extensions must include the full stop.
func HasExtensions(file fs.File, exts ...string) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}

	return slices.Contains(exts, path.Ext(info.Name()))
}

// AsyncWriter is a helper function for creating a read stream for the return values of Encoders
func AsyncWriter(fn func(w io.WriteCloser) error, errs chan<- error) io.ReadCloser {
	r, w := io.Pipe()
	go func(fn func(w io.WriteCloser) error, w io.WriteCloser, errs chan<- error) {
		if err := fn(w); err != nil {
			errs <- err
		}
	}(fn, w, errs)

	return r
}
