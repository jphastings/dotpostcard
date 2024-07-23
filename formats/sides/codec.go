package sides

import (
	"errors"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"

	_ "golang.org/x/image/tiff"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/metadata"
)

var (
	ErrIsMissingMetadata = errors.New("is missing its metadata file")
	ErrIsMissingFront    = errors.New("is missing its front image")
	ErrIsMissingBack     = errors.New("is missing its back image")
)

var _ formats.Bundle = bundle{}

type bundle struct {
	name       string
	frontFile  fs.File
	backFile   fs.File
	metaBundle formats.Bundle
}

var _ formats.Codec = codec{}

type codec struct{}

var usableExtensions = []string{".webp", ".png", ".jpg", ".jpeg", ".tif", ".tiff"}

func Codec() formats.Codec { return codec{} }

func findMeta(dir fs.FS, prefix string) (formats.Bundle, string, error) {
	metaFile, metaFilename := findFile(dir, prefix+"-meta", metadata.Extensions)
	if metaFile == nil {
		return nil, "", ErrIsMissingMetadata
	}

	metaBundle, err := metadata.BundleFromFile(metaFile)
	if err != nil {
		return nil, "", fmt.Errorf("metadata file for %s couldn't be loaded: %w", err)
	}

	return metaBundle, metaFilename, nil
}

func findFile(dir fs.FS, prefix string, exts []string) (fs.File, string) {
	for _, possExt := range exts {
		foundFilename := prefix + possExt
		if f, err := dir.Open(foundFilename); err == nil {
			return f, foundFilename
		}
	}
	return nil, ""
}