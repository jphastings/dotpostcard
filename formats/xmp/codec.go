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

func (c codec) Bundle(files []fs.File, _ fs.DirEntry) ([]formats.Bundle, []fs.File) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range files {
		if formats.HasExtensions(file, ".xmp") {
			bundles = append(bundles, bundle{file})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining
}

func (c codec) Encode(pc types.Postcard, errs chan<- error) []io.ReadCloser {
	r := formats.AsyncWriter(func(w io.WriteCloser) error {
		if xmp, err := metadataToXMP(pc.Meta); err == nil {
			_, writeErr := w.Write(xmp)
			return writeErr
		} else {
			return err
		}
	}, errs)

	return []io.ReadCloser{r}
}

func (b bundle) Decode() (types.Postcard, error) {
	return types.Postcard{}, fmt.Errorf("decoding XMP files isn't implemented yet")
}
