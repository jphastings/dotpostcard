package component

import (
	"fmt"

	"github.com/jphastings/dotpostcard/types"
)

func validateMetadata(pc types.Postcard) error {
	if pc.Sides() == 2 {
		if pc.Meta.Flip == types.FlipNone || !pc.Meta.Flip.IsValid() {
			var validFlips string
			for _, f := range types.ValidFlips {
				if f != types.FlipNone {
					validFlips += ", " + string(f)
				}
			}

			return fmt.Errorf(
				"invalid flip type for two-sided postcard '%s' must be one of: %s",
				pc.Meta.Flip,
				validFlips[2:],
			)
		}
	}

	return nil
}
