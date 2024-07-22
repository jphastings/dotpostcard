package testhelpers

import (
	"io/fs"
	"testing/fstest"
)

// TestFiles makes an in-memory filesystem suitable for testing Codec.Bundle.
// The files within contain no data or context, but have the supplied names.
func TestFiles(filenames []string, alsoInDir ...string) ([]fs.File, fs.ReadDirFS) {
	var files []fs.File
	dir := make(fstest.MapFS)

	for _, filename := range alsoInDir {
		dir[filename] = &fstest.MapFile{}
	}

	for _, filename := range filenames {
		dir[filename] = &fstest.MapFile{}

		f, err := dir.Open(filename)
		if err != nil {
			panic("couldn't open in memory file in test set up")
		}

		files = append(files, f)
	}

	return files, dir
}
