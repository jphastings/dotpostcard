package web

import (
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
)

func BundleFromReader(r io.Reader, refPath string) formats.Bundle {
	return bundle{
		Reader:  r,
		name:    strings.TrimSuffix(path.Base(refPath), path.Ext(refPath)),
		refPath: refPath,
	}
}

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	var finalErr error

	for _, file := range group.Files {
		// Assume the 'sides' bundler is run before this one, so all image files can be greedily assumed to be web format postcards
		filename, isImg := formats.HasFileSuffix(file, ".postcard", ".postcard.webp", ".webp")
		if !isImg {
			remaining = append(remaining, file)
			continue
		}

		bnd := bundle{
			Reader:  file,
			name:    strings.TrimSuffix(filename, path.Ext(filename)),
			refPath: path.Join(group.DirPath, filename),
		}

		bundles = append(bundles, bnd)
	}

	return bundles, remaining, finalErr
}

func (b bundle) RefPath() string {
	return b.refPath
}

func (b bundle) Name() string {
	return codecName
}
