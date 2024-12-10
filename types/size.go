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

// The fallback surface area of a postcard, if no physical size is provided, but one is needed
var assumedSurfaceArea = 10.5 * 14.8

// Returns the actual physical dimensions (in cm), or guesses at a physical size
// based on the assumption that most postcards are 6x4 inches.
func (s Size) MustPhysical() (float64, float64) {
	if s.HasPhysical() {
		w, _ := s.CmWidth.Float64()
		h, _ := s.CmHeight.Float64()
		return w, h
	}

	pxA := float64(s.PxWidth * s.PxHeight)
	ar := float64(s.PxWidth) / float64(s.PxHeight)
	res := assumedSurfaceArea / float64(pxA)

	cmH := math.Sqrt(res * pxA / ar)
	cmW := ar * cmH

	return cmW, cmH
}

func (s Size) HasPhysical() bool {
	return s.CmWidth != nil && s.CmHeight != nil
}

// SetResolution is a helper function for setting the physical dimensions using a Dots Per Centimetre resolution
func (s *Size) SetResolution(xRes *big.Rat, yRes *big.Rat) {
	if xRes == nil || yRes == nil {
		return
	}

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

type Orientation string

const (
	OrientationLandscape Orientation = "landscape"
	OrientationPortrait  Orientation = "portrait"
	OrientationSquare    Orientation = "square"
)

// Returns the most probable orientation of the size. Within 5mm/5px of square is considered square
func (s Size) Orientation() Orientation {
	if s.HasPhysical() {
		diff, _ := new(big.Rat).Sub(s.CmWidth, s.CmHeight).Float64()
		if math.Abs(diff) <= 0.5 {
			return OrientationSquare
		}
	} else {
		diff := math.Abs(float64(s.PxWidth - s.PxHeight))
		if math.Abs(diff) <= 5 {
			return OrientationSquare
		}
	}

	if s.PxWidth > s.PxHeight {
		return OrientationLandscape
	}
	return OrientationPortrait
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
