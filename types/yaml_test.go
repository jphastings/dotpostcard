package types_test

import (
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func Test_YAMLUnmarshal(t *testing.T) {
	input := testhelpers.SampleYAML
	expected := testhelpers.SamplePostcard.Meta
	// Pixel dimensions are set by the image, not by metadata, so ignore these
	expected.Physical.FrontDimensions.PxWidth = 0
	expected.Physical.FrontDimensions.PxHeight = 0

	var meta types.Metadata
	assert.NoError(t, yaml.Unmarshal(input, &meta))
	assert.Equal(t, expected, meta)

	// Box secrets are unmarshalled as polygon secrets
	boxSecret := `type: box
width: 0.1
height: 0.2
left: 0.3
top: 0.6`

	var poly types.Polygon
	assert.NoError(t, yaml.Unmarshal([]byte(boxSecret), &poly))
	assert.Equal(t, expected.Front.Secrets[0], poly)
}

func Test_YAMLMarshal(t *testing.T) {
	input := testhelpers.SamplePostcard.Meta

	yamlBytes, err := yaml.Marshal(input)
	assert.NoError(t, err)

	// Compare by unmarshaling both and checking the resulting structs,
	// which is more resilient to formatting differences (e.g., "x" vs "×")
	var actualMeta, expectedMeta types.Metadata
	assert.NoError(t, yaml.Unmarshal(yamlBytes, &actualMeta))
	assert.NoError(t, yaml.Unmarshal(testhelpers.SampleYAML, &expectedMeta))

	assert.Equal(t, expectedMeta, actualMeta)
}
