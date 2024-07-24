package formats

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/jphastings/postcards/types"
)

type EncodeOptions struct {
	// Creates archival quality postcard files; this may require some *upsampling*, depending on the requested format
	Archival bool
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
	Encode(types.Postcard, EncodeOptions) []FileWriter

	// Name is the human usable name of the codec
	Name() string
}

type FileWriter struct {
	r        io.ReadCloser
	filename string
	Err      error
}

// NewFileWriter is a helper function for creating a read stream for the return values of Encoders
func NewFileWriter(filename string, fn func(w io.Writer) error) FileWriter {
	r, w := io.Pipe()

	fw := FileWriter{
		filename: filename,
		r:        r,
	}

	go func(fn func(w io.Writer) error, w io.WriteCloser) {
		if err := fn(w); err != nil {
			fw.Err = errors.Join(fw.Err, err)
		}
		if err := w.Close(); err != nil {
			fw.Err = errors.Join(fw.Err, err)
		}
	}(fn, w)

	return fw
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

	_, err = io.Copy(f, fw.r)
	return fw.filename, errors.Join(fw.Err, err)
}

func (fw FileWriter) Bytes() ([]byte, error) {
	data, err := io.ReadAll(fw.r)
	return data, errors.Join(fw.Err, err)
}

func (fw FileWriter) WriteTo(w io.Writer) error {
	_, err := io.Copy(w, fw.r)
	return err
}
