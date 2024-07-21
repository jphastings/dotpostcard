package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
	"gopkg.in/yaml.v3"
)

var _ formats.Bundle = bundle{}

type bundle struct {
	ext  MetadataType
	file fs.File
}

var _ formats.Codec = codec{}

type codec struct{ ext MetadataType }

type MetadataType string

var AsJSON MetadataType = ".json"
var AsYAML MetadataType = ".yaml"

func Codec(ext MetadataType) formats.Codec { return codec{ext: ext} }

func (c codec) Bundle(files []fs.File, _ fs.DirEntry) ([]formats.Bundle, []fs.File) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range files {
		if formats.HasExtensions(file, string(c.ext)) {
			bundles = append(bundles, bundle{file: file, ext: c.ext})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining
}

// The structure information is stored in the internal/types/postcard.go file, because Go.
func (c codec) Encode(pc types.Postcard, errs chan<- error) []formats.FileWriter {
	name := fmt.Sprintf("%s-meta%s", pc.Name, c.ext)
	writer := func(w io.WriteCloser) error {
		switch c.ext {
		case AsJSON:
			return json.NewEncoder(w).Encode(pc)
		case AsYAML:
			return yaml.NewEncoder(w).Encode(pc)
		default:
			return fmt.Errorf("unknown metadata format '%s'", c.ext)
		}
	}

	return []formats.FileWriter{formats.NewFileWriter(name, writer, errs)}
}

func (b bundle) Decode() (types.Postcard, error) {
	var pc types.Postcard
	switch b.ext {
	case AsJSON:
		err := json.NewDecoder(b.file).Decode(&pc)
		return pc, err
	case AsYAML:
		err := yaml.NewDecoder(b.file).Decode(&pc)
		return pc, err
	default:
		return types.Postcard{}, fmt.Errorf("unknown metadata format '%s'", b.ext)
	}
}
