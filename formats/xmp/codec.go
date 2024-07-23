package xmp

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
)

var _ formats.Bundle = bundle{}

type bundle struct {
	referenceFilename string

	r io.Reader
}

var _ formats.Codec = codec{}

type codec struct{}

func Codec() formats.Codec { return codec{} }

// BundleFromBytes allows decoding of an XMP data block stored within another file format
func BundleFromBytes(data []byte) formats.Bundle {
	return bundle{r: bytes.NewReader(data)}
}

func (c codec) Bundle(files []fs.File, _ fs.FS) ([]formats.Bundle, []fs.File, map[string]error) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range files {
		if filename, ok := formats.HasFileSuffix(file, ".xmp"); ok {
			bundles = append(bundles, bundle{r: file, referenceFilename: filename})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining, make(map[string]error)
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

func (b bundle) Decode() (types.Postcard, error) {
	return types.Postcard{}, fmt.Errorf("decoding XMP files isn't implemented yet")
}

func (b bundle) ReferenceFilename() string {
	return b.referenceFilename
}
