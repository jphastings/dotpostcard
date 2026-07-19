package appcore

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/jphastings/dotpostcard/types"
	"gopkg.in/yaml.v3"
)

// MetaJSONFromCardBytes extracts the embedded XMP metadata from a compiled
// web-format postcard file's raw bytes (webp/jpeg/png) and returns it as
// canonical metadata JSON, in the same shape CardFile.MetaJSON returns and
// CompilePostcard accepts back as its metaJSON parameter. filename is used
// the way OpenCardFile uses its path argument — for error context, since
// mimetype is sniffed from data — and need not be a real filesystem path.
// Pixel data is never decoded.
func MetaJSONFromCardBytes(filename string, data []byte) (string, error) {
	meta, _, err := decodeCardMeta(filename, data)
	if err != nil {
		return "", err
	}

	return metadataJSON(meta)
}

// MetaJSONFromComponentYAML decodes a "{name}-meta.yaml" component sidecar's
// bytes — the same YAML shape formats/metadata's codec reads — into
// types.Metadata, and returns it as canonical metadata JSON (the same shape
// MetaJSONFromCardBytes and CardFile.MetaJSON return).
func MetaJSONFromComponentYAML(yamlData []byte) (string, error) {
	var meta types.Metadata
	if err := yaml.NewDecoder(bytes.NewReader(yamlData)).Decode(&meta); err != nil {
		return "", fmt.Errorf("decoding metadata YAML: %w", err)
	}

	return metadataJSON(meta)
}

// ComponentYAMLFromMetaJSON is MetaJSONFromComponentYAML's inverse: it takes
// canonical metadata JSON (the shape CompilePostcard accepts and MetaJSON()
// emits, secrets carrying their "type" discriminator) and returns the bytes
// of a "{name}-meta.yaml" component sidecar, encoded via formats/metadata's
// own YAML codec — the same one the CLI's "-f yaml" output goes through — so
// the result is indistinguishable from a CLI-written sidecar. The macOS app
// calls this after compiling a card that originated from component files, to
// write its (possibly edited) metadata back alongside the source images.
func ComponentYAMLFromMetaJSON(metaJSON string) ([]byte, error) {
	var meta types.Metadata
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return nil, fmt.Errorf("parsing postcard metadata: %w", err)
	}

	fws, err := metadata.Codec(metadata.AsYAML).Encode(types.Postcard{Meta: meta}, &formats.EncodeOptions{})
	if err != nil {
		return nil, fmt.Errorf("encoding metadata YAML: %w", err)
	}
	if len(fws) == 0 {
		return nil, fmt.Errorf("encoding metadata YAML: codec produced no output file")
	}

	return fws[0].Bytes()
}
