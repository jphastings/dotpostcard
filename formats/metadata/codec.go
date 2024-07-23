package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"

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

var Extensions = []string{".json", ".yaml", ".yml"}

func Codec(ext MetadataType) formats.Codec { return codec{ext: ext} }

func BundleFromFile(file fs.File) (formats.Bundle, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	ext := path.Ext(info.Name())
	if !slices.Contains(Extensions, ext) {
		return nil, fmt.Errorf("unknown metadata extension '%s'", ext)
	}

	return bundle{file: file, ext: MetadataType(ext)}, nil
}

func (c codec) Bundle(files []fs.File, _ fs.FS) ([]formats.Bundle, []fs.File, map[string]error) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range files {
		if formats.HasFileSuffix(file, string(c.ext)) {
			bundles = append(bundles, bundle{file: file, ext: c.ext})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining, make(map[string]error)
}

// The structure information is stored in the internal/types/postcard.go file, because Go.
func (c codec) Encode(pc types.Postcard, _ formats.EncodeOptions, errs chan<- error) []formats.FileWriter {
	name := fmt.Sprintf("%s-meta%s", pc.Name, c.ext)
	writer := func(w io.Writer) error {
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
		err := json.NewDecoder(b.file).Decode(&pc.Meta)
		return pc, err
	case AsYAML:
		err := yaml.NewDecoder(b.file).Decode(&pc.Meta)
		return pc, err
	default:
		return types.Postcard{}, fmt.Errorf("unknown metadata format '%s'", b.ext)
	}
}
