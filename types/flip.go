package types

import (
	"fmt"
	"slices"
)

type Flip string

const (
	FlipBook      Flip = "book"
	FlipLeftHand  Flip = "left-hand"
	FlipCalendar  Flip = "calendar"
	FlipRightHand Flip = "right-hand"
	FlipNone      Flip = ""
)

var ValidFlips = []Flip{FlipBook, FlipCalendar, FlipLeftHand, FlipRightHand, FlipNone}

// Heteroriented will be true if the card need to pivot about a diagonal axis for the front and back to remain upright.
// the negation of this method is always whether the card is homoriented or not.
func (flip Flip) IsHeteroriented() bool {
	return flip == FlipLeftHand || flip == FlipRightHand
}

// IsValid will return false if the flip string isn't a known one
func (flip Flip) IsValid() bool {
	return slices.Contains(ValidFlips, flip)
}

func (flip Flip) Description() string {
	switch flip {
	case FlipBook:
		return "vertical axis (like a book)"
	case FlipLeftHand:
		return "diagonal (up-right) axis (flipping with your left hand)"
	case FlipCalendar:
		return "horizontal axis (like a calendar)"
	case FlipRightHand:
		return "diagonal (down-right) axis (flipping with your right hand)"
	case FlipNone:
		return "one sided"
	default:
		panic("unknown flip axis")
	}
}

// Checks whether the given Flip is appropriate for sides with the provided dimensions
func CheckFlip(front, back Size, flip Flip) error {
	fo := front.Orientation()
	bo := back.Orientation()
	switch {
	case fo == OrientationSquare:
		// Any flip is permissable
		return nil
	case fo == bo:
		if !flip.IsHeteroriented() {
			return nil
		}
		return fmt.Errorf("the front and back images are both %s, but '%s' flip only works with sides of different orientations. Try '%s' or '%s'.", fo, flip, FlipBook, FlipCalendar)
	case fo != bo:
		if flip.IsHeteroriented() {
			return nil
		}
		return fmt.Errorf("the front (%s) and back (%s) images aren't the same orientation, but '%s' flip only works with sides of the same orientation. Try '%s' or '%s'.", fo, bo, flip, FlipLeftHand, FlipRightHand)
	}

	return nil
}
