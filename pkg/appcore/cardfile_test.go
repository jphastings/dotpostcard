package appcore

import (
	"testing"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

func TestOpenCardFileSingleExtensionFilename(t *testing.T) {
	data, _ := encodePostcard(t, "bare-single-ext-card")
	path := writeBareFile(t, "bare-single-ext-card.postcard", data)

	cf, err := OpenCardFile(path)
	assert.NoError(t, err)

	assert.Equal(t, "bare-single-ext-card", cf.Name())
	assert.Equal(t, path, cf.Path())
	assert.Equal(t, types.FlipBook, cf.meta.Flip)

	img, err := cf.Image()
	assert.NoError(t, err)
	assert.Equal(t, data, img)

	summaryJSON, err := cf.SummaryJSON()
	assert.NoError(t, err)
	assert.Contains(t, summaryJSON, `"mimetype":"image/jpeg"`)
}
