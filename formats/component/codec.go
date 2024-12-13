package component

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/jphastings/dotpostcard/types"
)

const codecName = "Component files"

var (
	ErrIsMissingMetadata = errors.New("is missing its metadata file")
	ErrIsMissingFront    = errors.New("is missing its front image")
	ErrIsMissingBack     = errors.New("is missing its back image")
)

var _ formats.Bundle = bundle{}

type decoder interface {
	Decode(formats.DecodeOptions) (types.Postcard, error)
}

type bundle struct {
	name       string
	refPath    string
	frontFile  io.ReadCloser
	backFile   io.ReadCloser
	metaBundle decoder
}

var _ formats.Codec = codec{}

type codec struct {
}

func Codec() formats.Codec { return codec{} }

func (c codec) Name() string { return codecName }

func findMeta(dir fs.FS, name string, dirPath string) (formats.Bundle, string, error) {
	metaFile, metaFilename := findFile(dir, name+"-meta", metadata.Extensions)
	if metaFile == nil {
		return nil, "", ErrIsMissingMetadata
	}

	metaBundle, err := metadata.BundleFromFile(metaFile, dirPath)
	if err != nil {
		return nil, "", fmt.Errorf("metadata file for %s couldn't be loaded: %w", name, err)
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
