package appcore

import (
	"bytes"
	"fmt"

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
