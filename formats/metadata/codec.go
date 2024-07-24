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

const codecName = "Metadata"

var _ formats.Bundle = bundle{}

type bundle struct {
	ext     MetadataType
	file    fs.File
	refPath string
}

var _ formats.Codec = codec{}

type codec struct{ ext MetadataType }

type MetadataType string

var AsJSON MetadataType = ".json"
var AsYAML MetadataType = ".yaml"

var Extensions = []string{".json", ".yaml", ".yml"}

func Codec(ext MetadataType) formats.Codec { return codec{ext: ext} }

func (c codec) Name() string {
	switch c.ext {
	case AsJSON:
		return "JSON " + codecName
	case AsYAML:
		return "YAML " + codecName
	default:
		return codecName
	}
}

func BundleFromFile(file fs.File, dirPath string) (formats.Bundle, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	filename := info.Name()
	ext := path.Ext(filename)
	if !slices.Contains(Extensions, ext) {
		return nil, fmt.Errorf("unknown metadata extension '%s'", ext)
	}

	return bundle{file: file, ext: MetadataType(ext), refPath: path.Join(dirPath, filename)}, nil
}

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range group.Files {
		if filename, ok := formats.HasFileSuffix(file, string(c.ext)); ok {
			bundles = append(bundles, bundle{file: file, ext: c.ext, refPath: path.Join(group.DirPath, filename)})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining, nil
}

// The structure information is stored in the internal/types/postcard.go file, because Go.
func (c codec) Encode(pc types.Postcard, _ formats.EncodeOptions) []formats.FileWriter {
	name := fmt.Sprintf("%s-meta%s", pc.Name, c.ext)
	writer := func(w io.Writer) error {
		switch c.ext {
		case AsJSON:
			return json.NewEncoder(w).Encode(pc.Meta)
		case AsYAML:
			return yaml.NewEncoder(w).Encode(pc.Meta)
		default:
			return fmt.Errorf("unknown metadata format '%s'", c.ext)
		}
	}

	return []formats.FileWriter{formats.NewFileWriter(name, writer)}
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

func (b bundle) RefPath() string {
	return b.refPath
}

func (b bundle) Name() string {
	return codecName
}
