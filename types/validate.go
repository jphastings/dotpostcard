package types

import (
	"fmt"
)

func (pc Postcard) Validate() error {
	switch pc.Sides() {
	case 0:
		return fmt.Errorf("a postcard must have at least a front side")
	case 1:
		if pc.Meta.Flip != FlipNone {
			return fmt.Errorf("flip of '%s' given, but only 1 side provided", pc.Meta.Flip)
		}
	case 2:
		if pc.Meta.Flip == FlipNone || !pc.Meta.Flip.IsValid() {
			return fmt.Errorf("invalid flip type '%s' for two-sided postcard", pc.Meta.Flip)
		}
	}

	return nil
}
