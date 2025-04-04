package images_test

import (
	"testing"

	"github.com/jphastings/dotpostcard/internal/geom3d"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestOutline(t *testing.T) {
	im := testhelpers.TestImages["sample-transparency-front.png"]
	points, err := images.Outline(im, false, false)

	assert.NoError(t, err)
	for _, p := range points {
		assert.GreaterOrEqual(t, p.X, 0.0)
		assert.LessOrEqual(t, p.X, 1.0)
		assert.GreaterOrEqual(t, p.Y, 0.0)
		assert.LessOrEqual(t, p.Y, 1.0)
	}

	area := geom3d.Area(points)
	assert.Negativef(t, area, "the outline is not wound anticlockwise")
	assert.InDelta(t, -1.55, area, 0.02, "the produced outline points produce an unexpected shape")
}
