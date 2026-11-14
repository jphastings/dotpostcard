package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlipTurnsClockwiseThroughTheCycle(t *testing.T) {
	assert.Equal(t, FlipLeftHand, FlipBook.Turned(1))
	assert.Equal(t, FlipCalendar, FlipLeftHand.Turned(1))
	assert.Equal(t, FlipRightHand, FlipCalendar.Turned(1))
	assert.Equal(t, FlipBook, FlipRightHand.Turned(1))
	assert.Equal(t, FlipRightHand, FlipBook.Turned(-1))
	assert.Equal(t, FlipCalendar, FlipBook.Turned(6))
	assert.Equal(t, FlipNone, FlipNone.Turned(1))
}

func TestPointTurnsClockwise(t *testing.T) {
	p := Point{X: 0.25, Y: 0.1}
	assert.Equal(t, Point{X: 0.9, Y: 0.25}, p.Turned(1))
	assert.InDeltaSlice(t, []float64{0.75, 0.9}, []float64{p.Turned(2).X, p.Turned(2).Y}, 1e-9)
	assert.Equal(t, p.Turned(3), p.Turned(-1))
}

func TestMetadataTurnSwapsDimensionsOnOddFrontTurnsAndTurnsEachSidesSecrets(t *testing.T) {
	secret := func() []Polygon { return []Polygon{{Points: []Point{{X: 0.25, Y: 0.1}}}} }
	newMeta := func() Metadata {
		m := Metadata{Flip: FlipBook, Front: Side{Secrets: secret()}, Back: Side{Secrets: secret()}}
		m.Physical.FrontDimensions = Size{PxWidth: 30, PxHeight: 20, CmWidth: big.NewRat(15, 1), CmHeight: big.NewRat(10, 1)}
		return m
	}

	odd := newMeta()
	odd.Turn(1, 0)
	assert.Equal(t, 20, odd.Physical.FrontDimensions.PxWidth)
	assert.Equal(t, 30, odd.Physical.FrontDimensions.PxHeight)
	assert.Equal(t, 0, odd.Physical.FrontDimensions.CmWidth.Cmp(big.NewRat(10, 1)))
	assert.Equal(t, FlipLeftHand, odd.Flip)
	assert.Equal(t, secret()[0].Points[0].Turned(1), odd.Front.Secrets[0].Points[0])
	assert.Equal(t, secret()[0].Points[0], odd.Back.Secrets[0].Points[0])

	even := newMeta()
	even.Turn(2, 1)
	assert.Equal(t, 30, even.Physical.FrontDimensions.PxWidth)
	assert.Equal(t, secret()[0].Points[0].Turned(2), even.Front.Secrets[0].Points[0])
	assert.Equal(t, secret()[0].Points[0].Turned(1), even.Back.Secrets[0].Points[0])
}
