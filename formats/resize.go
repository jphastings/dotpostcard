package formats

import (
	"image"
	"math"
)

const defaultMaxSide = 1536

// DetermineSize will calculate the appropriate width and height for the postcard one or two sides given.
// Aspect ratio will be maintained (including for heteroriented sides)
// Archival: the largest dimensions of the given sides will be used
// Non-archival: A maximum of DefaultMaxSide will be used, with the shorter side scaled accordingly
func DetermineSize(opts *EncodeOptions, front image.Image, back image.Image) (frontSize, finalSize image.Rectangle) {
	frontSize = front.Bounds()
	frontLandscape := frontSize.Max.X > frontSize.Max.Y

	finalSize = frontSize
	var backSize image.Rectangle
	var homoriented bool

	// Upsize if back has more pixels than the front; but ignore if there's only one side
	if back != nil {
		backSize = back.Bounds()
		homoriented = (backSize.Max.X > backSize.Max.Y) == frontLandscape

		if homoriented {
			if backSize.Max.X > finalSize.Max.X {
				finalSize.Max.X = backSize.Max.X
				finalSize.Max.Y = backSize.Max.Y
			}
		} else {
			if backSize.Max.X > finalSize.Max.Y {
				finalSize.Max.X = backSize.Max.Y
				finalSize.Max.Y = backSize.Max.X
			}
		}
	}

	finalAR := float64(finalSize.Max.X) / float64(finalSize.Max.Y)

	// Downscale if not archival
	if opts == nil || !opts.Archival {
		maxSize := defaultMaxSide
		if opts != nil && opts.MaxDimension > 0 {
			maxSize = opts.MaxDimension
		}
		if frontLandscape {
			if finalSize.Max.X > maxSize {
				finalSize.Max.X = maxSize
				finalSize.Max.Y = int(math.Floor(float64(maxSize) / finalAR))
			}
		} else {
			if finalSize.Max.Y > maxSize {
				finalSize.Max.X = int(math.Floor(float64(maxSize) * finalAR))
				finalSize.Max.Y = maxSize
			}
		}
	}

	return frontSize, finalSize
}
