package fastfoto

import (
	"image"
	"math"
)

// The model places a secret's edge to within a couple of percent of the side, which is enough
// to leave half a line of an address readable or to cut into the name above it. Each edge is
// moved, within these limits, to the emptiest strip nearby: the gap between lines of writing.
const (
	snapOutward   = 0.035
	snapInward    = 0.015
	snapTolerance = 0.003
	// edgeMargin is the strip of card always left between a secret and the card's edge, so
	// hiding the secret never flattens the edge itself.
	edgeMargin = 0.005
)

// snapSecrets moves each horizontal or vertical edge of the polygons (points are fractions of
// img) to the clearest strip of card near it, preferring the outermost when several are clear,
// then pulls every point back inside the card's edge.
func snapSecrets(img image.Image, polys [][][2]float64) [][][2]float64 {
	out := make([][][2]float64, len(polys))
	for n, poly := range polys {
		var area float64
		for i, a := range poly {
			b := poly[(i+1)%len(poly)]
			area += a[0]*b[1] - b[0]*a[1]
		}
		winding := math.Copysign(1, area)

		snapped := append([][2]float64(nil), poly...)
		for i, a := range poly {
			j := (i + 1) % len(poly)
			b := poly[j]
			dx, dy := b[0]-a[0], b[1]-a[1]
			// ponytail: slanted edges are left where the model put them; snap along their
			// normal if the model starts drawing them.
			switch {
			case dy == 0 && dx != 0:
				y := clearest(img, 1, a[1], a[0], b[0], -math.Copysign(1, dx)*winding)
				snapped[i][1], snapped[j][1] = y, y
			case dx == 0 && dy != 0:
				x := clearest(img, 0, a[0], a[1], b[1], math.Copysign(1, dy)*winding)
				snapped[i][0], snapped[j][0] = x, x
			}
		}
		lo, hi := [2]float64{1, 1}, [2]float64{}
		for _, p := range snapped {
			for axis := range p {
				lo[axis], hi[axis] = min(lo[axis], p[axis]), max(hi[axis], p[axis])
			}
		}
		for axis := range lo {
			other := 1 - axis
			nearest := edgeDepth(img, axis, false, lo[other], hi[other]) + edgeMargin
			furthest := 1 - edgeDepth(img, axis, true, lo[other], hi[other]) - edgeMargin
			for i := range snapped {
				snapped[i][axis] = min(max(snapped[i][axis], nearest), furthest)
			}
		}
		out[n] = snapped
	}
	return out
}

// clearest gives where, near pos on the axis, a line spanning from..to on the other axis
// crosses the least ink. outward is +1 or -1: the direction that grows the polygon.
func clearest(img image.Image, axis int, pos, from, to, outward float64) float64 {
	b := img.Bounds()
	size, span := b.Dy(), b.Dx()
	if axis == 0 {
		size, span = span, size
	}
	lo, hi := int(min(from, to)*float64(span)), int(max(from, to)*float64(span))
	hi = min(hi, span-1)
	step := max((hi-lo)/800, 1)

	type strip struct{ at, ink float64 }
	var strips []strip
	least := math.Inf(1)
	for t := -snapInward; t <= snapOutward; t += 2 / float64(size) {
		at := min(max(pos+outward*t, 0), 1)
		px := min(int(at*float64(size)), size-1)
		var ink float64
		var samples int
		for s := lo; s <= hi; s += step {
			x, y := b.Min.X+s, b.Min.Y+px
			if axis == 0 {
				x, y = b.Min.X+px, b.Min.Y+s
			}
			// Premultiplied, so pixels off the card's edge count as clear rather than black.
			r, g, bl, a := img.At(x, y).RGBA()
			ink += float64(a) - (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl))
			samples++
		}
		if samples == 0 {
			return pos
		}
		ink /= float64(samples) * 0xffff
		strips = append(strips, strip{at, ink})
		least = min(least, ink)
	}
	best := pos
	for _, s := range strips {
		if s.ink <= least+snapTolerance {
			best = s.at
		}
	}
	return best
}

// edgeDepth gives how far in from the image's border (its far border when far is set) the card
// starts on the axis, at its deepest across from..to on the other axis.
func edgeDepth(img image.Image, axis int, far bool, from, to float64) float64 {
	b := img.Bounds()
	size, span := b.Dx(), b.Dy()
	if axis == 1 {
		size, span = span, size
	}
	lo, hi := int(from*float64(span)), min(int(to*float64(span)), span-1)
	step := max((hi-lo)/400, 1)
	deepest := 0
	for s := lo; s <= hi; s += step {
		d := 0
		for ; d < size/10; d++ {
			p := d
			if far {
				p = size - 1 - d
			}
			x, y := b.Min.X+p, b.Min.Y+s
			if axis == 1 {
				x, y = b.Min.X+s, b.Min.Y+p
			}
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x7fff {
				break
			}
		}
		deepest = max(deepest, d)
	}
	return float64(deepest) / float64(size)
}
