package formats

import (
	"io/fs"
	"strings"

	"github.com/jphastings/postcards/types"
)

// HasFileSuffix returns true if the given file has any one of the provided filename
// suffixes.
func HasFileSuffix(file fs.File, suffixes ...string) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}

	filename := info.Name()
	for _, suffix := range suffixes {
		if strings.HasSuffix(filename, suffix) {
			return true
		}
	}

	return false
}

// Bundle represents a bundle of files that will be decoded together
type Bundle interface {
	// Decode must select the first single/set of postcard file(s) in 'input'
	Decode() (pc types.Postcard, err error)
}
