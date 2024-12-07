package xmp

import (
	"strconv"

	"github.com/jphastings/dotpostcard/types"
)

func extractSecrets(flip types.Flip, regions []xmpRegion) (front, back []types.Polygon) {
	for _, region := range regions {
		// Secrets are alwats polygonal & relative shapes, so skip any others
		if region.Boundary.Shape != "polygon" || region.Boundary.Unit != "relative" {
			continue
		}

		// Secret regions have specific names (in various languages), skip any that don't have the right name
		isSecretRegion := false
		for _, n := range region.Names {
			// Check for any of the indicators that this is a secret region
			if val, ok := privateExplainer[n.Lang]; ok && val == n.Value {
				isSecretRegion = true
				break
			}
		}
		if !isSecretRegion {
			continue
		}

		regionOnFront := true
		poly := types.Polygon{Prehidden: true}
		for i, vert := range region.Boundary.Vertices {
			x, xErr := strconv.ParseFloat(vert.X, 64)
			y, yErr := strconv.ParseFloat(vert.Y, 64)
			if xErr != nil || yErr != nil {
				poly.Points = nil
				break
			}

			// Make sure the region is only one one side of the card (if it's a double sided card)
			if flip != types.FlipNone {
				vertOnFront := y < 0.5
				if i == 0 {
					regionOnFront = vertOnFront
				} else if vertOnFront != regionOnFront {
					poly.Points = nil
					break
				}
			}

			point := types.Point{X: x, Y: y}.TransformToSingleSided(regionOnFront, flip)
			poly.Points = append(poly.Points, point)
		}
		if poly.Points == nil {
			continue
		}

		if regionOnFront {
			front = append(front, poly)
		} else {
			back = append(back, poly)
		}
	}

	return front, back
}
