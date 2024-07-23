package sides

import (
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/jphastings/postcards/formats"
)

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

		ext := path.Ext(filename)
		if !slices.Contains(usableExtensions, ext) {
			remaining = append(remaining, file)
			continue
		}

		parts := strings.Split(strings.TrimSuffix(filename, ext), "-")
		splitN := len(parts) - 1
		prefix := strings.Join(parts[:splitN], "-")
		suffix := parts[splitN]
		if splitN == 0 {
			remaining = append(remaining, file)
			continue
		}
		if !slices.Contains([]string{"front", "back", "only"}, suffix) {
			remaining = append(remaining, file)
			continue
		}

		metaBundle, metaFilename, err := findMeta(dir, prefix)
		if err != nil {
			errs[filename] = err
			continue
		}
		skip = append(skip, metaFilename)

		switch suffix {
		case "only":
			bundles = append(bundles, bundle{
				name:       prefix,
				frontFile:  file,
				metaBundle: metaBundle,
			})

		case "front":
			backFile, _ := findFile(dir, prefix+"-back", usableExtensions)
			if backFile == nil {
				errs[filename] = ErrIsMissingBack
				continue
			}
			bundles = append(bundles, bundle{
				name:       prefix,
				frontFile:  file,
				backFile:   backFile,
				metaBundle: metaBundle,
			})

		case "back":
			frontFile, _ := findFile(dir, prefix+"-front", usableExtensions)
			if frontFile == nil {
				errs[filename] = ErrIsMissingFront
				continue
			}
			bundles = append(bundles, bundle{
				name:       prefix,
				frontFile:  frontFile,
				backFile:   file,
				metaBundle: metaBundle,
			})
		}
	}

	return bundles, remaining, errs
}
