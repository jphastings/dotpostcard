package formats

import (
	"io/fs"
	"strings"

	"github.com/jphastings/postcards/types"
)

// HasFileSuffix returns true if the given file has any one of the provided filename
// suffixes.
func HasFileSuffix(file fs.File, suffixes ...string) (string, bool) {
	info, err := file.Stat()
	if err != nil {
		return "", false
	}

	filename := info.Name()
	for _, suffix := range suffixes {
		if strings.HasSuffix(filename, suffix) {
			return filename, true
		}
	}

	return filename, false
}

// Bundle represents a bundle of files that will be decoded together
type Bundle interface {
	// Decode must select the first single/set of postcard file(s) in 'input'
	Decode() (pc types.Postcard, err error)
	// ReferenceFilename is at least one filename from the bundle, usable in feedback to the user
	ReferenceFilename() string
}
