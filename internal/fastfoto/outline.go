package fastfoto

import (
	"image"
	"math"
)

const (
	// A side's stock counts as light where it's this much brighter than the backing. Low,
	// so faint fibres on a light card's torn edge still count as card.
	lightMargin      = 3
	minLightFraction = 0.6
	// Card sampled this far inside the box, past any shadow the box may have swallowed.
	lightInsets = 3

	// Each side's canvas comes from its own geometry estimate, so the two differ by a few
	// pixels of offset, a fraction of a degree of tilt and a fraction of a percent of feed length.
	registerShiftPx       = 30.0
	boundaryPoints        = 4000
	minSetPoints          = 40
	maxRegisterResidualPx = 4.0
	searchStepPx          = 0.5
	// A registration that moves no corner this far isn't worth the blur of redrawing the canvas.
	negligiblePx = 1.0
)

// lightFraction is how much of the card just inside the box's edges is brighter than the backing.
func lightFraction(sig *image.Gray, box image.Rectangle) float64 {
	light, n := 0, 0
	probe := func(x, y int) {
		n++
		if int(sig.Pix[y*sig.Stride+x])-128 > lightMargin {
			light++
		}
	}
	for k := 0; k < lightInsets; k++ {
		inset := 40 + 20*k
		for t := 0.1; t <= 0.9; t += 0.02 {
			x, y := box.Min.X+int(t*float64(box.Dx())), box.Min.Y+int(t*float64(box.Dy()))
			probe(x, box.Min.Y+inset)
			probe(x, box.Max.Y-1-inset)
			probe(box.Min.X+inset, y)
			probe(box.Max.X-1-inset, y)
		}
	}
	return float64(light) / float64(n)
}

// lightGuide is 128 plus how much brighter than the backing each pixel is. Shadow and dark
// card both read as backing, so only light stock counts as card. Inside the card's box that
// is judged on the mixed signal, because ink can be darker than the backing yet warmer than
// it. Beyond the box there is only backing and shadow, which can be redder when darker, so
// only brightness counts there.
func lightGuide(mixed, lum *image.Gray, box image.Rectangle) *image.Gray {
	g := image.NewGray(mixed.Rect)
	for y := 0; y < mixed.Rect.Dy(); y++ {
		for x := 0; x < mixed.Rect.Dx(); x++ {
			sig := lum
			if (image.Point{x, y}).In(box) {
				sig = mixed
			}
			g.Pix[y*g.Stride+x] = 128 + uint8(max(int(sig.Pix[y*sig.Stride+x])-128-lightMargin, 0))
		}
	}
	return g
}

// registration says where an aligned canvas's pixel p comes from in the canvas drawn so far:
// p stretched along the feed by 1+ey about the centre, turned by deg (clockwise), then
// shifted by dx, dy. Two passes of the same card through the scanner differ in feed length by
// a fraction of a percent, so that matters as much as shift and turn. Across the feed the
// sensor's pixels are fixed, so there's no stretch to find.
type registration struct{ deg, dx, dy, ey float64 }

// reach is how far the registration moves the canvas's corners.
func (r registration) reach(w, h int) float64 {
	theta, far := r.deg*math.Pi/180, 0.0
	for _, c := range [4]point{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		u, v := c.x*float64(w)/2, c.y*float64(h)/2
		far = math.Max(far, math.Hypot(r.dx-theta*v, r.dy+theta*u+r.ey*v))
	}
	return far
}

// mirrorAlpha is a side's alpha as the other side sees it: a book flip mirrors left to right.
func mirrorAlpha(from *image.NRGBA) *image.Gray {
	w, h := from.Bounds().Dx(), from.Bounds().Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Pix[y*w+x] = from.Pix[y*from.Stride+4*(w-1-x)+3]
		}
	}
	return out
}

// A run of the outline's boundary facing one way, and where the other side's own edge lies
// from it, along that facing.
type edgeSet struct {
	pts     []point
	vertic  bool    // faces up or down, so it measures a vertical displacement
	outward float64 // which way the boundary lies from its pixels: half a pixel, up, down, left or right
	offset  float64
	ok      bool
}

// register finds how to move the other side's canvas so the outline lies on its own
// strongest edges. It only moves the canvas; the outline's shape is never altered.
//
// Each stretch of the outline's four edges is slid along its normal to the other side's
// strongest edge, then a shift, turn and feed stretch are fitted to those offsets. Fitting the
// offsets, rather than searching for one transform, lets shadows that mislead the same way on
// opposite edges cancel out instead of pulling the outline wider.
func register(outline *image.Gray, other *image.NRGBA) registration {
	w, h := outline.Bounds().Dx(), outline.Bounds().Dy()
	var facing [4][]point // up, down, left, right
	b := outlineBounds(outline)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			if outline.Pix[i] < 128 {
				continue
			}
			p := point{float64(x), float64(y)}
			for k, outside := range [4]int{i - w, i + w, i - 1, i + 1} {
				if outline.Pix[outside] < 128 {
					facing[k] = append(facing[k], p)
				}
			}
		}
	}

	strength := edgeMap(other)
	var sets []edgeSet
	for k, pts := range facing {
		vertic := k < 2
		along := func(p point) float64 {
			if vertic {
				return p.x
			}
			return p.y
		}
		lo, hi := float64(b.Min.X), float64(b.Max.X)
		if !vertic {
			lo, hi = float64(b.Min.Y), float64(b.Max.Y)
		}
		lo, hi = lo+edgeMiddle*(hi-lo), hi-edgeMiddle*(hi-lo)
		var halves [2][]point
		for _, p := range pts {
			if a := along(p); a >= lo && a <= hi {
				halves[int(2*(a-lo)/(hi-lo+1))] = append(halves[int(2*(a-lo)/(hi-lo+1))], p)
			}
		}
		for _, half := range halves {
			if len(half) > boundaryPoints/8 {
				half = thin(half, boundaryPoints/8)
			}
			sets = append(sets, edgeSet{pts: half, vertic: vertic, outward: [4]float64{-0.5, 0.5, -0.5, 0.5}[k]})
		}
	}
	for i := range sets {
		sets[i].slide(strength)
	}
	return fitRegistration(sets, float64(w-1)/2, float64(h-1)/2)
}

func outlineBounds(outline *image.Gray) image.Rectangle {
	b := outline.Bounds()
	r := image.Rectangle{Min: b.Max, Max: b.Min}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if outline.Pix[y*outline.Stride+x] >= 128 {
				r.Min.X, r.Min.Y = min(r.Min.X, x), min(r.Min.Y, y)
				r.Max.X, r.Max.Y = max(r.Max.X, x+1), max(r.Max.Y, y+1)
			}
		}
	}
	return r
}

func thin(pts []point, n int) []point {
	out := make([]point, n)
	for i := range out {
		out[i] = pts[i*len(pts)/n]
	}
	return out
}

// slide finds the offset, along the set's axis, that puts it on the strongest edges.
func (s *edgeSet) slide(strength *image.Gray) {
	if len(s.pts) < minSetPoints {
		return
	}
	score := func(d float64) float64 {
		total := 0.0
		for _, p := range s.pts {
			if s.vertic {
				total += sampleGray(strength, p.x, p.y+s.outward+d)
			} else {
				total += sampleGray(strength, p.x+s.outward+d, p.y)
			}
		}
		return total
	}
	var scores []float64
	best := 0
	for d := -registerShiftPx; d <= registerShiftPx; d += searchStepPx {
		scores = append(scores, score(d))
		if scores[len(scores)-1] > scores[best] {
			best = len(scores) - 1
		}
	}
	s.offset, s.ok = -registerShiftPx+searchStepPx*float64(best), true
	if best > 0 && best < len(scores)-1 {
		// The peak's top lies between samples: fit a parabola through it and its neighbours.
		below, above := scores[best]-scores[best-1], scores[best]-scores[best+1]
		if below+above > 0 {
			s.offset += searchStepPx * (below - above) / (2 * (below + above))
		}
	}
}

// fitRegistration is the least-squares shift, turn and feed stretch that explain the sets'
// offsets. A set that's far worse than the rest is dropped as a misread edge, but not when
// every set is about equally off, which is shadow misleading them all.
func fitRegistration(sets []edgeSet, cx, cy float64) registration {
	for {
		// Unknowns: dx, dy, turn, ey. Vertical offsets read dy + turn*u + ey*v, horizontal
		// ones dx - turn*v, for (u, v) the set's centre from the canvas's.
		var rows [][5]float64
		var owners []int
		for i, s := range sets {
			if !s.ok {
				continue
			}
			var mid point
			for _, p := range s.pts {
				mid.x, mid.y = mid.x+p.x/float64(len(s.pts)), mid.y+p.y/float64(len(s.pts))
			}
			u, v := mid.x-cx, mid.y-cy
			if s.vertic {
				rows = append(rows, [5]float64{0, 1, u, v, s.offset})
			} else {
				rows = append(rows, [5]float64{1, 0, -v, 0, s.offset})
			}
			owners = append(owners, i)
		}
		if len(rows) < 6 {
			return registration{}
		}
		x, ok := leastSquares(rows)
		if !ok {
			return registration{}
		}
		residuals := make([]float64, len(rows))
		worst := 0
		for r, row := range rows {
			residuals[r] = math.Abs(row[0]*x[0] + row[1]*x[1] + row[2]*x[2] + row[3]*x[3] - row[4])
			if residuals[r] > residuals[worst] {
				worst = r
			}
		}
		typical := quantile(append([]float64(nil), residuals...), 0.5)
		if residuals[worst] <= 2*typical+maxRegisterResidualPx {
			return registration{deg: x[2] * 180 / math.Pi, dx: x[0], dy: x[1], ey: x[3]}
		}
		sets[owners[worst]].ok = false
	}
}

// leastSquares solves the normal equations of rows of four coefficients and a target.
func leastSquares(rows [][5]float64) (x [4]float64, ok bool) {
	var m [4][5]float64
	for _, r := range rows {
		for i := 0; i < 4; i++ {
			for j := 0; j < 5; j++ {
				m[i][j] += r[i] * r[j]
			}
		}
	}
	for i := 0; i < 4; i++ {
		pivot := i
		for r := i + 1; r < 4; r++ {
			if math.Abs(m[r][i]) > math.Abs(m[pivot][i]) {
				pivot = r
			}
		}
		if math.Abs(m[pivot][i]) < 1e-6 {
			return x, false
		}
		m[i], m[pivot] = m[pivot], m[i]
		for r := 0; r < 4; r++ {
			if r != i {
				f := m[r][i] / m[i][i]
				for c := i; c < 5; c++ {
					m[r][c] -= f * m[i][c]
				}
			}
		}
	}
	for i := range x {
		x[i] = m[i][4] / m[i][i]
	}
	return x, true
}

// edgeMap is the strength of steps in luminance or in colour (red minus blue, which can
// part stock from the backing where luminance can't). It compares block averages a few pixels
// apart, so halftone dots and noise don't read as edges, only a card's sharp edge.
func edgeMap(img *image.NRGBA) *image.Gray {
	const r = 2
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var planes [2][]float32
	for k := range planes {
		planes[k] = make([]float32, w*h)
	}
	for i := 0; i < w*h; i++ {
		p := img.Pix[(i/w)*img.Stride+4*(i%w):]
		planes[0][i] = float32(0.299*float64(p[0]) + 0.587*float64(p[1]) + 0.114*float64(p[2]))
		planes[1][i] = 2 * float32(int(p[0])-int(p[2]))
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	across := make([]float32, w*h) // each pixel's mean with its horizontal neighbours
	for _, plane := range planes {
		for y := 0; y < h; y++ {
			for x := r; x < w-r; x++ {
				var sum float32
				for k := -r; k <= r; k++ {
					sum += plane[y*w+x+k]
				}
				across[y*w+x] = sum / (2*r + 1)
			}
		}
		block := func(x, y int) float32 {
			var sum float32
			for k := -r; k <= r; k++ {
				sum += across[(y+k)*w+x]
			}
			return sum / (2*r + 1)
		}
		for y := 2 * r; y < h-2*r; y++ {
			for x := 2 * r; x < w-2*r; x++ {
				g := math.Abs(float64(block(x+r, y)-block(x-r, y))) + math.Abs(float64(block(x, y+r)-block(x, y-r)))
				out.Pix[y*w+x] = max(out.Pix[y*w+x], uint8(min(g, 255)))
			}
		}
	}
	return out
}

// sampleAlpha is bilinear, with the plane's surroundings fully transparent.
func sampleAlpha(g *image.Gray, x, y float64) float64 {
	w, h := g.Bounds().Dx(), g.Bounds().Dy()
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	at := func(dx, dy int) float64 {
		px, py := int(x0)+dx, int(y0)+dy
		if px < 0 || py < 0 || px >= w || py >= h {
			return 0
		}
		return float64(g.Pix[py*g.Stride+px])
	}
	return at(0, 0)*(1-fx)*(1-fy) + at(1, 0)*fx*(1-fy) + at(0, 1)*(1-fx)*fy + at(1, 1)*fx*fy
}
