package testhelpers

import (
	"fmt"
	"io/fs"
	"testing/fstest"

	"github.com/jphastings/postcards/formats"
)

func DataForTestFile(filename string) []byte {
	return []byte(fmt.Sprintf("Data for %s", filename))
}

// TestFiles makes an in-memory filesystem suitable for testing Codec.Bundle.
// The files within contain no data or context, but have the supplied names.
func TestFiles(filenames []string, alsoInDir ...string) formats.FileGroup {
	var files []fs.File
	dir := make(fstest.MapFS)

	for _, filename := range alsoInDir {
		dir[filename] = &fstest.MapFile{}
	}

	for _, filename := range filenames {
		dir[filename] = &fstest.MapFile{
			Data: DataForTestFile(filename),
		}

		f, err := dir.Open(filename)
		if err != nil {
			panic("couldn't open in memory file in test set up")
		}

		files = append(files, f)
	}

	return formats.FileGroup{
		Files: files,
		Dir:   dir,
	}
}

func Filenames(files []fs.File) []string {
	filenames := make([]string, len(files))
	for i, f := range files {
		info, err := f.Stat()
		if err != nil {
			panic("could not find filename of test fs.File; this shouldn't happen")
		}

		filenames[i] = info.Name()
	}

	return filenames
}
