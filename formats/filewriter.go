package formats

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/jphastings/dotpostcard/types"
)

type EncodeOptions struct {
	// Creates archival quality postcard files; this may require some *upsampling*, depending on the requested format
	Archival bool
	// Indicates the maximum width or height (in pixels) a postcard should have. Will be ignored if Archival is true
	MaxDimension int
}

type DecodeOptions struct {
	// Assumes the postcard was on uniformly coloured paper when it was scanned, and attempts to convert it to transparency.
	RemoveBorder bool
}

// FileGroup represents a subset of files within a single directory
type FileGroup struct {
	Files []fs.File
	Dir   fs.FS
	// The path on the filesystem of the directory, if a filesystem was the source
	DirPath string
}

// Codec structs hold mechanisms for storing and reading postcard information in a specific format
type Codec interface {
	// Bundle must extract any single/set of postcard file(s) that can be decoded by this codec
	// from the given input files (which will all be in the same directory), including any directly associated
	Bundle(FileGroup) ([]Bundle, []fs.File, error)

	// Encode must produce any files needed to represent postcards in this format.
	Encode(types.Postcard, *EncodeOptions) []FileWriter

	// Name is the human usable name of the codec
	Name() string
}

type FileWriter struct {
	fn       func(io.Writer) error
	filename string
}

// NewFileWriter is a helper function for creating a read stream for the return values of Encoders
func NewFileWriter(filename string, fn func(w io.Writer) error) FileWriter {
	return FileWriter{
		filename: filename,
		fn:       fn,
	}
}

func (fw FileWriter) WriteFile(dir string, overwrite bool) (string, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if !overwrite {
		flags |= os.O_EXCL
	}

	f, err := os.OpenFile(path.Join(dir, fw.filename), flags, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := fw.fn(f); err != nil {
		return "", err
	}

	return fw.filename, nil
}

func (fw FileWriter) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := fw.fn(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (fw FileWriter) WriteTo(w io.Writer) error { return fw.fn(w) }
