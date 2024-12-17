package component

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/jphastings/dotpostcard/types"
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
		if b.frontFile == nil {
			var toSkip string
			b.frontFile, toSkip = findFile(group.Dir, b.name+"-only", usableExtensions)
			skip = append(skip, toSkip)
			skipBack = true
		}
		if b.frontFile == nil {
			err = fmt.Errorf("no image found for the front of this postcard (%s-front.<ext>, or %s-only.<ext>)", b.name, b.name)
			finalErr = errors.Join(finalErr, formats.NewFileError(filename, err))
			continue
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

type decodedMeta types.Metadata

func (m decodedMeta) Decode(formats.DecodeOptions) (types.Postcard, error) {
	return types.Postcard{Meta: types.Metadata(m)}, nil
}

func BundleFromReaders(meta types.Metadata, front io.ReadCloser, back ...io.ReadCloser) formats.Bundle {
	b := bundle{
		frontFile: front,
		name:      meta.Name,
		// TODO: is this right?
		refPath:    ".",
		metaBundle: decodedMeta(meta),
	}

	if len(back) >= 1 {
		b.backFile = back[0]
	}

	return b
}
