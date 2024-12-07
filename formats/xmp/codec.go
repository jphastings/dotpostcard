package xmp

import (
	"bytes"
	"io"
	"io/fs"
	"path"

	"github.com/jphastings/dotpostcard/formats"
)

const codecName = "XMP Metadata"

var _ formats.Bundle = bundle{}

type bundle struct {
	refPath string
	r       io.Reader
}

var _ formats.Codec = codec{}

type codec struct{}

func Codec() formats.Codec { return codec{} }

func (c codec) Name() string { return codecName }

// BundleFromBytes allows decoding of an XMP data block stored within another file format
func BundleFromBytes(data []byte, refPath string) formats.Bundle {
	return bundle{r: bytes.NewReader(data), refPath: refPath}
}

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range group.Files {
		if filename, ok := formats.HasFileSuffix(file, ".xmp"); ok {
			bundles = append(bundles, bundle{r: file, refPath: path.Join(group.DirPath, filename)})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining, nil
}

func (b bundle) RefPath() string {
	return b.refPath
}

func (b bundle) Name() string {
	return codecName
}
