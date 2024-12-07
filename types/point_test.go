package types_test

import (
	"fmt"
	"testing"

	. "github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

var transformCases = []struct {
	onFront     bool
	flip        Flip
	transformed Point
}{
	{true, FlipNone, Point{X: 0.13, Y: 0.17}},

	{true, FlipBook, Point{X: 0.13, Y: 0.085}},
	{false, FlipBook, Point{X: 0.13, Y: 0.585}},

	{true, FlipCalendar, Point{X: 0.13, Y: 0.085}},
	{false, FlipCalendar, Point{X: 0.13, Y: 0.585}},

	{true, FlipLeftHand, Point{X: 0.13, Y: 0.085}},
	{false, FlipLeftHand, Point{X: 0.17, Y: 0.935}},

	{true, FlipRightHand, Point{X: 0.13, Y: 0.085}},
	{false, FlipRightHand, Point{X: 0.83, Y: 0.565}},
}

// Accuracy of floating point results
const acc = 0.0000000001

func TestTransformToDoubleSided(t *testing.T) {
	original := Point{X: 0.13, Y: 0.17}

	for _, c := range transformCases {
		t.Run(fmt.Sprintf("Front:%v-Flip:%s", c.onFront, c.flip), func(t *testing.T) {
			got := original.TransformToDoubleSided(c.onFront, c.flip)
			assert.InDelta(t, c.transformed.X, got.X, acc, "Incorrect X value: wanted %v got %v", c.transformed.X, got.X)
			assert.InDelta(t, c.transformed.Y, got.Y, acc, "Incorrect Y value: wanted %v got %v", c.transformed.X, got.Y)
		})
	}
}

func TestTransformToSingleSided(t *testing.T) {
	original := Point{X: 0.13, Y: 0.17}

	for _, c := range transformCases {
		t.Run(fmt.Sprintf("Front:%v-Flip:%s", c.onFront, c.flip), func(t *testing.T) {
			got := c.transformed.TransformToSingleSided(c.onFront, c.flip)
			assert.InDelta(t, original.X, got.X, acc, "Incorrect X value: wanted %v got %v", original.X, got.X)
			assert.InDelta(t, original.Y, got.Y, acc, "Incorrect Y value: wanted %v got %v", original.X, got.Y)
		})
	}
}
