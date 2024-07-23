package web

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/chai2010/webp"
	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/xmp"
)

func (c codec) Bundle(files []fs.File, dir fs.FS) ([]formats.Bundle, []fs.File, map[string]error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	errs := make(map[string]error)

	for _, file := range files {
		if !formats.HasFileSuffix(file, ".webp") {
			remaining = append(remaining, file)
		}
		info, err := file.Stat()
		if err != nil {
			continue
		}

		filename := info.Name()

		data, err := io.ReadAll(file)
		if err != nil {
			errs[filename] = fmt.Errorf("couldn't read file to determine if it is a postcard image: %w", err)
			continue
		}

		xmpData, err := webp.GetMetadata(data, "xmp")
		if err != nil {
			errs[filename] = fmt.Errorf("couldn't read file to determine if it is a postcard image: %w", err)
			continue
		}

		pc, err := xmp.BundleFromBytes(xmpData).Decode()
		if err != nil {
			errs[filename] = fmt.Errorf("didn't contain postcard metadata: %w", err)
			continue
		}

		pc.Name = strings.TrimSuffix(filename, path.Ext(filename))

		bnd := bundle{Reader: file, postcard: pc}

		bundles = append(bundles, bnd)
	}

	return bundles, remaining, errs
}
