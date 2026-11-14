package fastfoto

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretEdgesSettleBetweenLinesAndInsideTheCardEdge(t *testing.T) {
	card := image.NewNRGBA(image.Rect(0, 0, 1000, 1000))
	draw.Draw(card, image.Rect(20, 20, 980, 980), image.White, image.Point{}, draw.Src)
	for _, y := range []int{300, 400, 500} { // the name, then two address lines
		draw.Draw(card, image.Rect(500, y, 980, y+40), image.NewUniform(color.Black), image.Point{}, draw.Src)
	}

	// As a model might draw it: starting halfway down the first address line and running off the card.
	got := snapSecrets(card, [][][2]float64{{{0.5, 0.42}, {1, 0.42}, {1, 0.56}, {0.5, 0.56}}})[0]

	top, right, bottom := got[0][1], got[1][0], got[2][1]
	assert.Greater(t, top, 0.34, "the name stays visible")
	assert.Less(t, top, 0.40, "the first address line is covered to its full height")
	assert.GreaterOrEqual(t, bottom, 0.54, "the last address line is covered")
	assert.Less(t, right, 0.98, "the card's own edge is never hidden")
	assert.Greater(t, right, 0.95)
}
