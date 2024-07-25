package xmp

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
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

func (c codec) Encode(pc types.Postcard, _ formats.EncodeOptions) []formats.FileWriter {
	filename := fmt.Sprintf("%s-meta.xmp", pc.Name)
	writer := func(w io.Writer) error {
		// Don't write pixel & physical size information to an XMP which isn't embedded
		if xmp, err := MetadataToXMP(pc.Meta, nil); err == nil {
			_, writeErr := w.Write(xmp)
			return writeErr
		} else {
			return err
		}
	}
	fw := formats.NewFileWriter(filename, writer)

	return []formats.FileWriter{fw}
}

func (b bundle) Decode(_ formats.DecodeOptions) (types.Postcard, error) {
	return types.Postcard{}, fmt.Errorf("decoding XMP files isn't implemented yet")
}

func (b bundle) RefPath() string {
	return b.refPath
}

func (b bundle) Name() string {
	return codecName
}
