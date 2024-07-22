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

type bundle struct{ io.Reader }

var _ formats.Codec = codec{}

type codec struct{}

func Codec() formats.Codec { return codec{} }

// BundleFromBytes allows decoding of an XMP data block stored within another file format
func BundleFromBytes(data []byte) formats.Bundle {
	return bundle{bytes.NewReader(data)}
}

func (c codec) Bundle(files []fs.File, _ fs.ReadDirFS) ([]formats.Bundle, []fs.File, map[string]error) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range files {
		if formats.HasFileSuffix(file, ".xmp") {
			bundles = append(bundles, bundle{file})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining, make(map[string]error)
}

func (c codec) Encode(pc types.Postcard, _ formats.EncodeOptions, errs chan<- error) []formats.FileWriter {
	filename := fmt.Sprintf("%s-meta.xmp", pc.Name)
	writer := func(w io.Writer) error {
		if xmp, err := metadataToXMP(pc.Meta); err == nil {
			_, writeErr := w.Write(xmp)
			return writeErr
		} else {
			return err
		}
	}
	fw := formats.NewFileWriter(filename, writer, errs)

	return []formats.FileWriter{fw}
}

func (b bundle) Decode() (types.Postcard, error) {
	return types.Postcard{}, fmt.Errorf("decoding XMP files isn't implemented yet")
}
