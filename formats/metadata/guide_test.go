package metadata_test

import (
	_ "embed"
	"testing"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

//go:embed guide-meta.yaml
var guideYAML []byte

// Ensures that the guide YAML is valid
func Test_GuideYAML(t *testing.T) {
	var meta types.Metadata
	assert.NoError(t, yaml.Unmarshal(guideYAML, &meta))
}
