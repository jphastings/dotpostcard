package component

import (
	"errors"
	"io/fs"
	"path"
	"regexp"
	"slices"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/metadata"
)

var usableExtensions = []string{".webp", ".png", ".jpg", ".jpeg", ".tif", ".tiff"}
var bundleRE = regexp.MustCompile(`^(.+)-(?:(front|back|only)\.(?:webp|png|jpe?g|tiff?)|meta\.(?:yaml|json))$`)

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	var finalErr error

	var skip []string
	for _, file := range group.Files {
		info, err := file.Stat()
		if err != nil {
			continue
		}
		filename := info.Name()
		if slices.Contains(skip, filename) {
			continue
		}

		match := bundleRE.FindStringSubmatch(filename)
		if len(match) == 0 {
			remaining = append(remaining, file)
			continue
		}

		b := bundle{
			refPath: path.Join(group.DirPath, filename),
			name:    match[1],
		}
		skipBack := false

		switch match[2] {
		case "only":
			b.frontFile = file
			skipBack = true
		case "front":
			b.frontFile = file
		case "back":
			b.backFile = file
		case "": // This is a metadata file
			mf, err := metadata.BundleFromFile(file, group.DirPath)
			if err != nil {
				finalErr = errors.Join(finalErr, formats.NewFileError(filename, err))
				continue
			}
			b.metaBundle = mf
		}

		if b.frontFile == nil {
			var toSkip string
			b.frontFile, toSkip = findFile(group.Dir, b.name+"-front", usableExtensions)
			skip = append(skip, toSkip)
		}
		if b.backFile == nil && !skipBack {
			var toSkip string
			b.backFile, toSkip = findFile(group.Dir, b.name+"-back", usableExtensions)
			skip = append(skip, toSkip)
		}
		if b.metaBundle == nil {
			var toSkip string
			b.metaBundle, toSkip, err = findMeta(group.Dir, b.name, group.DirPath)
			if err != nil {
				finalErr = errors.Join(finalErr, formats.NewFileError(filename, err))
				continue
			}
			skip = append(skip, toSkip)
		}

		bundles = append(bundles, b)
	}

	return bundles, remaining, finalErr
}

func (b bundle) RefPath() string {
	return b.refPath
}

func (b bundle) Name() string {
	return codecName
}
