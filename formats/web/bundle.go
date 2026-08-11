package web

import (
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
)

// CardNameFromPath derives a postcard's name from the path of one of its files, stripping
// both the file's extension and a trailing ".postcard".
func CardNameFromPath(refPath string) string {
	return strings.TrimSuffix(strings.TrimSuffix(path.Base(refPath), path.Ext(refPath)), ".postcard")
}

func BundleFromReader(r io.ReadCloser, refPath string) formats.Bundle {
	return bundle{
		ReadCloser: r,
		name:       CardNameFromPath(refPath),
		refPath:    refPath,
	}
}

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	var finalErr error

	for _, file := range group.Files {
		// Assume the 'component' bundler is run before this one, so all image files can be greedily assumed to be web format postcards
		filename, isImg := formats.HasFileSuffix(file, ".postcard.webp", ".postcard.jpg", ".postcard.jpeg", ".postcard.png", ".postcard")
		if !isImg {
			remaining = append(remaining, file)
			continue
		}

		bnd := bundle{
			ReadCloser: file,
			name:       CardNameFromPath(filename),
			refPath:    path.Join(group.DirPath, filename),
		}

		bundles = append(bundles, bnd)
	}

	return bundles, remaining, finalErr
}

func (b bundle) RefPath() string {
	return b.refPath
}

func (b bundle) CardName() string {
	return b.name
}

func (b bundle) CodecName() string {
	return codecName
}
