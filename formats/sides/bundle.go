package sides

import (
	"io/fs"
	"regexp"
	"slices"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/metadata"
)

var usableExtensions = []string{".webp", ".png", ".jpg", ".jpeg", ".tif", ".tiff"}
var bundleRE = regexp.MustCompile(`^(.+)-(?:(front|back|only)\.(?:webp|png|jpe?g|tiff?)|meta\.(?:yaml|json))$`)

func (c codec) Bundle(files []fs.File, dir fs.FS) ([]formats.Bundle, []fs.File, map[string]error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	errs := make(map[string]error)

	var skip []string
	for _, file := range files {
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
			referenceFilename: filename,
			name:              match[1],
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
			mf, err := metadata.BundleFromFile(file)
			if err != nil {
				errs[filename] = err
				continue
			}
			b.metaBundle = mf
		}

		if b.frontFile == nil {
			var toSkip string
			b.frontFile, toSkip = findFile(dir, b.name+"-front", usableExtensions)
			skip = append(skip, toSkip)
		}
		if b.backFile == nil && !skipBack {
			var toSkip string
			b.backFile, toSkip = findFile(dir, b.name+"-back", usableExtensions)
			skip = append(skip, toSkip)
		}
		if b.metaBundle == nil {
			var toSkip string
			b.metaBundle, toSkip, err = findMeta(dir, b.name)
			if err != nil {
				errs[filename] = err
				continue
			}
			skip = append(skip, toSkip)
		}

		bundles = append(bundles, b)
	}

	return bundles, remaining, errs
}

func (b bundle) ReferenceFilename() string {
	return b.referenceFilename
}
