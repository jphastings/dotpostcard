package web

import (
	"io/fs"
	"path"
	"strings"

	"github.com/jphastings/postcards/formats"
)

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	var finalErr error

	for _, file := range group.Files {
		// Assume the 'sides' bundler is run before this one, so all webp files can be greedily assumed to be web format postcards
		filename, isWebp := formats.HasFileSuffix(file, ".postcard.webp", ".postcard", ".webp")
		if !isWebp {
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
