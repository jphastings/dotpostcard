package metadata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/jphastings/dotpostcard/types"
	"gopkg.in/yaml.v3"
)

const codecName = "Metadata"

//go:embed guide-meta.yaml
var GuideYAML string

var _ formats.Bundle = bundle{}

type bundle struct {
	mt      MetadataType
	file    fs.File
	refPath string
}

var _ formats.Codec = codec{}

type codec MetadataType

type MetadataType struct {
	Extension string
	Mimetype  string
	HumanName string
}

var AsJSON = MetadataType{".json", "application/json", "JSON " + codecName}
var AsYAML = MetadataType{".yaml", "application/yaml", "YAML " + codecName}
var AsXMP = MetadataType{".xmp", xmp.Mimetype, "XMP " + codecName}

var Extensions = []string{AsJSON.Extension, AsYAML.Extension, AsXMP.Extension}

func ExtToMediaType(ext string) (MetadataType, bool) {
	switch ext {
	case AsJSON.Extension:
		return AsJSON, true
	case AsYAML.Extension:
		return AsYAML, true
	case AsXMP.Extension:
		return AsXMP, true
	default:
		return MetadataType{}, false
	}
}

func Codec(mt MetadataType) formats.Codec { return codec(mt) }
func (c codec) Name() string              { return c.HumanName }

func BundleFromFile(file fs.File, dirPath string) (formats.Bundle, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	filename := info.Name()
	ext := path.Ext(filename)
	mt, ok := ExtToMediaType(ext)
	if !ok {
		return nil, fmt.Errorf("unknown metadata extension '%s'", ext)
	}

	return bundle{file: file, mt: mt, refPath: path.Join(dirPath, filename)}, nil
}

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File

	for _, file := range group.Files {
		if filename, ok := formats.HasFileSuffix(file, string(c.Extension)); ok {
			bundles = append(bundles, bundle{file: file, mt: MetadataType(c), refPath: path.Join(group.DirPath, filename)})
		} else {
			remaining = append(remaining, file)
		}
	}

	return bundles, remaining, nil
}

// The structure information is stored in the internal/types/postcard.go file, because Go.
func (c codec) Encode(pc types.Postcard, _ *formats.EncodeOptions) ([]formats.FileWriter, error) {
	name := fmt.Sprintf("%s-meta%s", pc.Name, c.Extension)
	writer := func(w io.Writer) error {
		switch MetadataType(c) {
		case AsJSON:
			return json.NewEncoder(w).Encode(pc.Meta)
		case AsYAML:
			return yaml.NewEncoder(w).Encode(pc.Meta)
		case AsXMP:
			xmp, err := xmp.MetadataToXMP(pc.Meta, nil)
			if err != nil {
				return err
			}
			_, err = w.Write(xmp)
			return err
		default:
			return fmt.Errorf("cannot encode '%s'", c.HumanName)
		}
	}

	return []formats.FileWriter{formats.NewFileWriter(name, c.Mimetype, writer)}, nil
}

func (b bundle) Decode(_ formats.DecodeOptions) (types.Postcard, error) {
	var pc types.Postcard
	var err error
	switch b.mt {
	case AsJSON:
		err = json.NewDecoder(b.file).Decode(&pc.Meta)
	case AsYAML:
		err = yaml.NewDecoder(b.file).Decode(&pc.Meta)
	case AsXMP:
		pc.Meta, err = xmp.MetadataFromXMP(b.file)
	default:
		return types.Postcard{}, fmt.Errorf("cannot decode '%s'", b.mt.HumanName)
	}

	pc.Name = strings.TrimSuffix(path.Base(b.refPath), "-meta"+string(b.mt.Extension))

	if err != nil {
		err = fmt.Errorf("error decoding %s: %w", b.refPath, err)
	}

	return pc, err
}

func (b bundle) RefPath() string {
	return b.refPath
}

func (b bundle) Name() string {
	return codecName
}
