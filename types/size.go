package types

import (
	"fmt"
	"math"
	"math/big"
)

const maxRatioDiff = 0.01

type Size struct {
	CmWidth  *big.Rat `json:"cmW,omitempty"`
	CmHeight *big.Rat `json:"cmH,omitempty"`
	PxWidth  int      `json:"pxW"`
	PxHeight int      `json:"pxH"`
}

func (s Size) HasPhysical() bool {
	return s.CmWidth != nil && s.CmHeight != nil
}

// SetResolution is a helper function for setting the physical dimensions using a Dots Per Centimetre resolution
func (s *Size) SetResolution(xRes *big.Rat, yRes *big.Rat) {
	s.CmWidth = (&big.Rat{}).Quo(
		big.NewRat(int64(s.PxWidth), 1),
		xRes,
	)
	s.CmHeight = (&big.Rat{}).Quo(
		big.NewRat(int64(s.PxHeight), 1),
		yRes,
	)
}

// Resolution returns the pixels per centimetre
func (s Size) Resolution() (xRes *big.Rat, yRes *big.Rat) {
	xRes = (&big.Rat{}).Quo(big.NewRat(int64(s.PxWidth), 1), s.CmWidth)
	yRes = (&big.Rat{}).Quo(big.NewRat(int64(s.PxHeight), 1), s.CmHeight)
	return
}

// SimilarPhysical compares two physical sizes and returns true if their dimensions are within ~1% of each other
func (s Size) SimilarPhysical(other Size, flip Flip) bool {
	if !s.HasPhysical() || !other.HasPhysical() {
		return true
	}

	if flip.IsHeteroriented() {
		return similar(s.CmWidth, other.CmHeight) && similar(s.CmHeight, other.CmWidth)
	} else {
		return similar(s.CmWidth, other.CmWidth) && similar(s.CmHeight, other.CmHeight)
	}
}

func similar(a, b *big.Rat) bool {
	ratio, _ := big.NewRat(1, 1).Quo(a, b).Float64()
	return math.Abs(1-ratio) <= maxRatioDiff
}

func (s Size) String() string {
	pxSize := fmt.Sprintf("%dpx x %dpx", s.PxWidth, s.PxHeight)
	if !s.HasPhysical() {
		return pxSize
	}

	fw, _ := s.CmWidth.Float64()
	fh, _ := s.CmHeight.Float64()

	return fmt.Sprintf(
		"%.1fcm x %.1fcm (%dpx x %dpx)",
		fw, fh,
		s.PxWidth, s.PxHeight,
	)
}
