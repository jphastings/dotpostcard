package fastfoto

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"math/big"
	"sort"

	"github.com/jphastings/dotpostcard/formats/component"
	"github.com/jphastings/dotpostcard/types"
)

const (
	// 600 dpi, in pixels per centimetre.
	pxPerCm  = 30000.0 / 127
	marginCm = 0.6

	// How far the upright box may poke past the frame before a corner counts as clipped.
	clipToleranceCm = 0.175

	// A card's edge points should hug its outline: when fewer do, something else is in the
	// feeder, like a second card shingled against it. Deep torn or pinked edges stay inside this.
	outlineToleranceCm = 0.5
	minOnOutline       = 0.85
	maxSideDifference  = 0.3 // cm

	// Trim keeps pixels at least this opaque. Low, so faint fibrous tails at the extremes
	// of the crop survive; a hard edge's matte ramps over only a few pixels.
	trimAlpha = 16

	// How far past a measured box a pixel may still be card, to cover the box's own error.
	boxTolerancePx = 2

	// Narrower than the margin a hardware-cropped frame guarantees at a tilted card's
	// outermost corner, so the strip never averages in card.
	stripPx    = 80
	latticePx  = 8
	scanStepPx = 3
	runSamples = 4
	edgeBandPx = 16
	// Outliers beyond this fraction of an edge's points don't push the box outward.
	extentTrim = 0.01
	// Corners are rounded or clipped, so tilt is fitted on the middle of each edge.
	edgeMiddle = 0.1
)

type rgb [3]float64

// frame is one decoded side as packed RGB, at the scanner's resolution.
type frame struct {
	w, h int
	pix  []uint8
}

func decodeFrame(r io.Reader) (*frame, error) {
	img, err := jpeg.Decode(r)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	f := &frame{w: b.Dx(), h: b.Dy(), pix: make([]uint8, 3*b.Dx()*b.Dy())}
	ycc, isYCC := img.(*image.YCbCr)
	for y := 0; y < f.h; y++ {
		for x := 0; x < f.w; x++ {
			i := 3 * (y*f.w + x)
			if isYCC {
				yi, ci := ycc.YOffset(b.Min.X+x, b.Min.Y+y), ycc.COffset(b.Min.X+x, b.Min.Y+y)
				f.pix[i], f.pix[i+1], f.pix[i+2] = color.YCbCrToRGB(ycc.Y[yi], ycc.Cb[ci], ycc.Cr[ci])
				continue
			}
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			f.pix[i], f.pix[i+1], f.pix[i+2] = uint8(r>>8), uint8(g>>8), uint8(bl>>8)
		}
	}
	return f, nil
}

func (f *frame) at(x, y int) rgb {
	x, y = min(max(x, 0), f.w-1), min(max(y, 0), f.h-1)
	i := 3 * (y*f.w + x)
	return rgb{float64(f.pix[i]), float64(f.pix[i+1]), float64(f.pix[i+2])}
}

// sample is bilinear, clamped to the frame's edge so no synthetic fill is ever invented.
func (f *frame) sample(x, y float64) rgb {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	var out rgb
	for _, c := range [4]struct {
		dx, dy int
		w      float64
	}{
		{0, 0, (1 - fx) * (1 - fy)}, {1, 0, fx * (1 - fy)}, {0, 1, (1 - fx) * fy}, {1, 1, fx * fy},
	} {
		p := f.at(int(x0)+c.dx, int(y0)+c.dy)
		for k := range out {
			out[k] += c.w * p[k]
		}
	}
	return out
}

func sampleGray(g *image.Gray, x, y float64) float64 {
	fx0, fy0 := math.Floor(x), math.Floor(y)
	x0, y0 := int(fx0), int(fy0)
	fx, fy := x-fx0, y-fy0
	at := func(dx, dy int) float64 {
		return float64(g.Pix[min(max(y0+dy, 0), g.Rect.Dy()-1)*g.Stride+min(max(x0+dx, 0), g.Rect.Dx()-1)])
	}
	return at(0, 0)*(1-fx)*(1-fy) + at(1, 0)*fx*(1-fy) + at(0, 1)*(1-fx)*fy + at(1, 1)*fx*fy
}

func (f *frame) contains(x, y float64) bool {
	return x >= 0 && y >= 0 && x <= float64(f.w-1) && y <= float64(f.h-1)
}

// backing is the colour of the scanner's backing plate for every row, measured in the
// strips either side of the card and interpolated across.
type backing struct {
	left, right []rgb
	w           int
}

func measureBacking(f *frame) backing {
	b := backing{left: make([]rgb, f.h), right: make([]rgb, f.h), w: f.w}
	mean := func(x0, y int) (m rgb) {
		for x := x0; x < x0+stripPx; x++ {
			p := f.at(x, y)
			for k := range m {
				m[k] += p[k] / stripPx
			}
		}
		return
	}
	for y := 0; y < f.h; y++ {
		b.left[y], b.right[y] = mean(8, y), mean(f.w-8-stripPx, y)
	}
	return b
}

func (b backing) at(x, y int) rgb {
	y = min(max(y, 0), len(b.left)-1)
	t := float64(x) / float64(b.w-1)
	var out rgb
	for k := range out {
		out[k] = b.left[y][k]*(1-t) + b.right[y][k]*t
	}
	return out
}

func luminance(c rgb) float64 { return 0.299*c[0] + 0.587*c[1] + 0.114*c[2] }

// contrast is how far a pixel is from the backing. Luminance can't separate some stock
// from the blue-grey backing, but red minus blue can. It's signed so paper reads brighter
// than backing and shadow darker, as the border detector expects.
func contrast(c, ref rgb) float64 {
	dY, dC := luminance(c)-luminance(ref), 2*((c[0]-c[2])-(ref[0]-ref[2]))
	if math.Abs(dC) > math.Abs(dY) {
		return dC
	}
	return dY
}

// newSignals is 128 plus each pixel's contrast against the backing, and the same on
// luminance alone. Only luminance can say a pixel is lighter than the backing: shadowed
// backing can be redder than unshadowed, which the mixed contrast reads as brighter.
func newSignals(f *frame) (mixed, lum *image.Gray) {
	bk := measureBacking(f)
	mixed, lum = image.NewGray(image.Rect(0, 0, f.w, f.h)), image.NewGray(image.Rect(0, 0, f.w, f.h))
	for y := 0; y < f.h; y++ {
		for x := 0; x < f.w; x++ {
			c, ref := f.at(x, y), bk.at(x, y)
			mixed.Pix[y*mixed.Stride+x] = clampByte(128 + contrast(c, ref))
			lum.Pix[y*lum.Stride+x] = clampByte(128 + luminance(c) - luminance(ref))
		}
	}
	return mixed, lum
}

func clampByte(v float64) uint8 { return uint8(math.Round(math.Min(255, math.Max(0, v)))) }

type point struct{ x, y float64 }

type edges struct{ left, right, top, bottom []point }

// firstRun walks from `from` towards `to` and returns where the first run of runSamples
// consecutive hits begins, so specks narrower than the run don't count.
func firstRun(from, to int, hit func(i int) bool) (int, bool) {
	step := scanStepPx
	if to < from {
		step = -step
	}
	run, start := 0, 0
	for i := from; (step > 0 && i < to) || (step < 0 && i > to); i += step {
		if !hit(i) {
			run = 0
			continue
		}
		if run == 0 {
			start = i
		}
		if run++; run == runSamples {
			return start, true
		}
	}
	return 0, false
}

// locate finds points along the card's four edges on a coarse lattice. Only top and
// bottom points where the card already touches the frame's edge are skipped, as they say
// nothing about the card; left and right points are kept either way.
func locate(sig *image.Gray, threshold float64) edges {
	w, h := sig.Bounds().Dx(), sig.Bounds().Dy()
	at := func(x, y int) float64 { return float64(sig.Pix[y*sig.Stride+x]) - 128 }
	// Some frames have a short dark band along their outermost rows or columns. Counting it
	// as card would put the edge on the frame boundary, so it is skipped when backing follows.
	// A bright run, or a dark one that outlasts the band, is still the card touching the edge.
	scan := func(from, to int, signal func(i int) float64) (int, bool) {
		dir := 1
		if to < from {
			dir = -1
		}
		start := from
		if signal(from) < -threshold {
			for d := 1; d <= edgeBandPx && from+dir*d != to; d++ {
				if v := signal(from + dir*d); v >= -threshold {
					if v <= threshold {
						start = from + dir*d
					}
					break
				}
			}
		}
		return firstRun(start, to, func(i int) bool { return math.Abs(signal(i)) > threshold })
	}
	var e edges
	for y := 0; y < h; y += latticePx {
		if x, ok := scan(0, w/2, func(x int) float64 { return at(x, y) }); ok {
			e.left = append(e.left, point{float64(x), float64(y)})
		}
		if x, ok := scan(w-1, w/2, func(x int) float64 { return at(x, y) }); ok {
			e.right = append(e.right, point{float64(x), float64(y)})
		}
	}
	for x := 0; x < w; x += latticePx {
		if y, ok := scan(0, h/2, func(y int) float64 { return at(x, y) }); ok && y >= scanStepPx {
			e.top = append(e.top, point{float64(x), float64(y)})
		}
		if y, ok := scan(h-1, h/2, func(y int) float64 { return at(x, y) }); ok && y <= h-1-scanStepPx {
			e.bottom = append(e.bottom, point{float64(x), float64(y)})
		}
	}
	return e
}

// fitSlope fits b = slope*a + intercept, then refits without points more than three
// standard deviations out, so a nick or speck on one edge can't tilt the whole line.
func fitSlope(a, b []float64) (float64, bool) {
	least := func(a, b []float64) (slope, intercept float64, ok bool) {
		n := float64(len(a))
		var sa, sb, saa, sab float64
		for i := range a {
			sa, sb = sa+a[i], sb+b[i]
			saa, sab = saa+a[i]*a[i], sab+a[i]*b[i]
		}
		if d := n*saa - sa*sa; d != 0 {
			slope = (n*sab - sa*sb) / d
			return slope, (sb - slope*sa) / n, true
		}
		return 0, 0, false
	}

	if len(a) < 32 {
		return 0, false
	}
	slope, intercept, ok := least(a, b)
	if !ok {
		return 0, false
	}
	var sumSq float64
	for i := range a {
		r := b[i] - (slope*a[i] + intercept)
		sumSq += r * r
	}
	limit := 3 * math.Sqrt(sumSq/float64(len(a)))
	if limit == 0 {
		return slope, true
	}
	var keptA, keptB []float64
	for i := range a {
		if math.Abs(b[i]-(slope*a[i]+intercept)) <= limit {
			keptA, keptB = append(keptA, a[i]), append(keptB, b[i])
		}
	}
	if len(keptA) < 32 {
		return slope, true
	}
	slope, _, ok = least(keptA, keptB)
	return slope, ok
}

// middle keeps the points whose position along the edge is away from its corners.
func middle(pts []point, along func(point) float64) []point {
	if len(pts) == 0 {
		return nil
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		lo, hi = math.Min(lo, along(p)), math.Max(hi, along(p))
	}
	var kept []point
	for _, p := range pts {
		if t := (along(p) - lo) / (hi - lo); t >= edgeMiddle && t <= 1-edgeMiddle {
			kept = append(kept, p)
		}
	}
	return kept
}

// tilt is the card's clockwise rotation in the frame: the mean of the middle two of the
// four edges' angles, so one bad edge can't skew it. Only measured; the card's shape is
// never altered to fit a line.
func (e edges) tilt() (float64, bool) {
	var angles []float64
	add := func(pts []point, along func(point) float64, across func(point) float64, sign float64) {
		pts = middle(pts, along)
		a, b := make([]float64, len(pts)), make([]float64, len(pts))
		for i, p := range pts {
			a[i], b[i] = along(p), across(p)
		}
		if slope, ok := fitSlope(a, b); ok {
			angles = append(angles, math.Atan(sign*slope))
		}
	}
	px, py := func(p point) float64 { return p.x }, func(p point) float64 { return p.y }
	add(e.top, px, py, 1)
	add(e.bottom, px, py, 1)
	add(e.left, py, px, -1)
	add(e.right, py, px, -1)
	if len(angles) == 0 {
		return 0, false
	}
	sort.Float64s(angles)
	if len(angles) >= 3 {
		angles = angles[1 : len(angles)-1]
	}
	var sum float64
	for _, a := range angles {
		sum += a
	}
	return sum / float64(len(angles)), true
}

// geometry is the card's upright box in coordinates rotated by theta about the frame's centre.
type geometry struct {
	theta, cx, cy          float64
	uMin, uMax, vMin, vMax float64
	missing                bool
	onOutline              float64 // fraction of edge points within outlineToleranceCm of their side's line
}

func (g geometry) width() float64  { return g.uMax - g.uMin }
func (g geometry) height() float64 { return g.vMax - g.vMin }

// rotated gives a frame point's coordinates along (u) and across (v) the card.
func (g geometry) rotated(x, y float64) (u, v float64) {
	dx, dy := x-g.cx, y-g.cy
	s, c := math.Sincos(g.theta)
	return dx*c + dy*s, -dx*s + dy*c
}

// unrotated is rotated's inverse.
func (g geometry) unrotated(u, v float64) (x, y float64) {
	s, c := math.Sincos(g.theta)
	return g.cx + u*c - v*s, g.cy + u*s + v*c
}

func quantile(vals []float64, q float64) float64 {
	sort.Float64s(vals)
	return vals[int(math.Round(q*float64(len(vals)-1)))]
}

func measure(sig *image.Gray, threshold float64) geometry {
	w, h := sig.Bounds().Dx(), sig.Bounds().Dy()
	e := locate(sig, threshold)
	theta, found := e.tilt()
	g := geometry{theta: theta, cx: float64(w) / 2, cy: float64(h) / 2, missing: !found}

	coord := func(pts []point, v func(u, v float64) float64) []float64 {
		out := make([]float64, len(pts))
		for i, p := range pts {
			out[i] = v(g.rotated(p.x, p.y))
		}
		return out
	}
	u := func(u, _ float64) float64 { return u }
	v := func(_, v float64) float64 { return v }

	// With no edge points the frame itself bounds the card in that direction.
	corners := [4]point{{0, 0}, {float64(w - 1), 0}, {0, float64(h - 1)}, {float64(w - 1), float64(h - 1)}}
	frameExtent := func(pick func(u, v float64) float64, wantMax bool) float64 {
		best := math.Inf(1)
		if wantMax {
			best = math.Inf(-1)
		}
		for _, c := range corners {
			val := pick(g.rotated(c.x, c.y))
			if wantMax {
				best = math.Max(best, val)
			} else {
				best = math.Min(best, val)
			}
		}
		return best
	}
	extent := func(pts []point, pick func(u, v float64) float64, wantMax bool) float64 {
		if len(pts) < 8 {
			g.missing = true
			return frameExtent(pick, wantMax)
		}
		if wantMax {
			return quantile(coord(pts, pick), 1-extentTrim)
		}
		return quantile(coord(pts, pick), extentTrim)
	}
	g.uMin, g.uMax = extent(e.left, u, false), extent(e.right, u, true)
	g.vMin, g.vMax = extent(e.top, v, false), extent(e.bottom, v, true)

	g.onOutline = 1
	alongX, alongY := func(p point) float64 { return p.x }, func(p point) float64 { return p.y }
	for _, side := range []struct {
		pts   []point
		along func(point) float64
		pick  func(u, v float64) float64
	}{{e.left, alongY, u}, {e.right, alongY, u}, {e.top, alongX, v}, {e.bottom, alongX, v}} {
		pts := middle(side.pts, side.along)
		if len(pts) < 8 {
			continue
		}
		vals := coord(pts, side.pick)
		line := quantile(append([]float64(nil), vals...), 0.5)
		near := 0
		for _, val := range vals {
			if math.Abs(val-line) <= outlineToleranceCm*pxPerCm {
				near++
			}
		}
		g.onOutline = math.Min(g.onOutline, float64(near)/float64(len(vals)))
	}
	return g
}

// clippedBy is how far, in pixels, the upright box's worst corner falls outside the frame:
// the card's corner was cut off by the scan.
func (g geometry) clippedBy(sig *image.Gray) float64 {
	w, h := sig.Bounds().Dx(), sig.Bounds().Dy()
	worst := 0.0
	for _, c := range [4]point{{g.uMin, g.vMin}, {g.uMax, g.vMin}, {g.uMin, g.vMax}, {g.uMax, g.vMax}} {
		x, y := g.unrotated(c.x, c.y)
		worst = math.Max(worst, math.Max(math.Max(-x, x-float64(w-1)), math.Max(-y, y-float64(h-1))))
	}
	return worst
}

// warp resamples the frame and its signal upright into canvases of the given size, with
// the card's box centred, then moves the canvas by reg. Canvas outside the frame reads as
// backing in the signal and is listed in offFrame, because what's drawn there is only the
// frame's edge repeated.
func (g geometry) warp(f *frame, sig, lum *image.Gray, w, h int, reg registration) (img *image.NRGBA, guide, lumGuide *image.Gray, offFrame []bool) {
	img, guide, lumGuide, offFrame = image.NewNRGBA(image.Rect(0, 0, w, h)), image.NewGray(image.Rect(0, 0, w, h)), image.NewGray(image.Rect(0, 0, w, h)), make([]bool, w*h)
	uc, vc := (g.uMin+g.uMax)/2, (g.vMin+g.vMax)/2
	// Both canvas and frame treat integer coordinates as pixel centres. Without tilt the
	// origin is rounded onto the frame's pixel grid so every pixel is copied, not blended.
	ox, oy := g.unrotated(uc-float64(w-1)/2, vc-float64(h-1)/2)
	if g.theta == 0 && reg == (registration{}) {
		ox, oy = math.Round(ox), math.Round(oy)
	}
	sin, cos := math.Sincos(g.theta)
	regSin, regCos := math.Sincos(reg.deg * math.Pi / 180)
	mx, my := float64(w-1)/2, float64(h-1)/2
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			u, v := float64(cx)-mx, float64(cy)-my
			v *= 1 + reg.ey
			qx, qy := mx+u*regCos-v*regSin+reg.dx, my+u*regSin+v*regCos+reg.dy
			x, y := ox+qx*cos-qy*sin, oy+qx*sin+qy*cos
			c := f.sample(x, y)
			i := cy*img.Stride + 4*cx
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(c[0]+0.5), uint8(c[1]+0.5), uint8(c[2]+0.5), 255
			guide.Pix[cy*guide.Stride+cx], lumGuide.Pix[cy*lumGuide.Stride+cx] = 128, 128
			if f.contains(x, y) {
				guide.Pix[cy*guide.Stride+cx] = clampByte(sampleGray(sig, x, y))
				lumGuide.Pix[cy*lumGuide.Stride+cx] = clampByte(sampleGray(lum, x, y))
			} else {
				offFrame[cy*w+cx] = true
			}
		}
	}
	return img, guide, lumGuide, offFrame
}

func clearOffFrame(img *image.NRGBA, offFrame []bool) {
	w := img.Bounds().Dx()
	for i, off := range offFrame {
		if off {
			img.Pix[(i/w)*img.Stride+4*(i%w)+3] = 0
		}
	}
}

// canvases are both sides warped upright onto canvases of one size, index 0 the front.
type canvases struct {
	w, h     int
	frames   [2]*frame
	frameSig [2]*image.Gray
	frameLum [2]*image.Gray
	geos     [2]geometry
	imgs     [2]*image.NRGBA
	signals  [2]*image.Gray
	lumSigs  [2]*image.Gray // luminance alone, for deciding what is lighter than the backing
	offFrame [2][]bool
	boxes    [2]image.Rectangle // each side's measured card box
}

func makeCanvases(frames [2]*frame, sigs, lums [2]*image.Gray, geos [2]geometry) canvases {
	margin := marginCm * pxPerCm
	cv := canvases{
		w:      int(math.Ceil(math.Max(geos[0].width(), geos[1].width()) + 2*margin)),
		h:      int(math.Ceil(math.Max(geos[0].height(), geos[1].height()) + 2*margin)),
		frames: frames, frameSig: sigs, frameLum: lums, geos: geos,
	}
	cx, cy := float64(cv.w-1)/2, float64(cv.h-1)/2
	for i := range frames {
		cv.rewarp(i, registration{})
		cv.boxes[i] = image.Rect(int(math.Round(cx-geos[i].width()/2)), int(math.Round(cy-geos[i].height()/2)),
			int(math.Round(cx+geos[i].width()/2)), int(math.Round(cy+geos[i].height()/2))).Intersect(image.Rect(0, 0, cv.w, cv.h))
	}
	return cv
}

// rewarp redraws one side's canvas moved by reg.
func (cv *canvases) rewarp(side int, reg registration) {
	cv.imgs[side], cv.signals[side], cv.lumSigs[side], cv.offFrame[side] = cv.geos[side].warp(cv.frames[side], cv.frameSig[side], cv.frameLum[side], cv.w, cv.h, reg)
}

func absGuide(sig *image.Gray) *image.Gray {
	g := image.NewGray(sig.Rect)
	for i, v := range sig.Pix {
		g.Pix[i] = clampByte(128 + math.Abs(float64(v)-128))
	}
	return g
}

// matted is both sides cut out of their canvases, before trimming.
type matted struct {
	front, back *image.NRGBA
	flags       []string
	turnDeg     [2]float64 // how far each side's canvas was turned to register it
}

// matte cuts both sides out. The side with the lighter stock is cut out against the backing,
// where shadow can never pass for card, and its alpha is carried over to the other side,
// whose canvas is registered to it, so both share one outline. With no light side the sides
// are each cut by how far they differ from the backing, and shadows may show.
func (cv canvases) matte() (matted, error) {
	light := [2]float64{lightFraction(cv.signals[0], cv.boxes[0]), lightFraction(cv.signals[1], cv.boxes[1])}
	lead := 1
	if light[0] > light[1]+0.05 {
		lead = 0
	}
	if light[lead] < minLightFraction {
		m0, m1, err := component.RemoveBorderPairGuided(cv.imgs[0], cv.imgs[1], absGuide(cv.signals[0]), absGuide(cv.signals[1]), false, pxPerCm, pxPerCm)
		if err != nil {
			return matted{}, err
		}
		out := matted{front: m0.(*image.NRGBA), back: m1.(*image.NRGBA),
			flags: []string{"neither side is lighter than the scanner's backing, so the card's edges may be uncertain: shadows can be mistaken for card"}}
		clearOffFrame(out.front, cv.offFrame[0])
		clearOffFrame(out.back, cv.offFrame[1])
		return out, nil
	}

	m, err := component.RemoveBorderGuided(cv.imgs[lead], lightGuide(cv.signals[lead], cv.lumSigs[lead], cv.boxes[lead].Inset(-boxTolerancePx)), pxPerCm)
	if err != nil {
		return matted{}, err
	}
	sides := [2]*image.NRGBA{}
	sides[lead] = m.(*image.NRGBA)
	clearOffFrame(sides[lead], cv.offFrame[lead])

	follow := 1 - lead
	outline := mirrorAlpha(sides[lead])
	reg := register(outline, cv.imgs[follow])
	if reg.reach(cv.w, cv.h) < negligiblePx {
		reg = registration{}
	}
	cv.rewarp(follow, reg)
	sides[follow] = cv.imgs[follow]
	for i, a := range outline.Pix {
		sides[follow].Pix[(i/outline.Stride)*sides[follow].Stride+4*(i%outline.Stride)+3] = a
	}
	clearOffFrame(sides[follow], cv.offFrame[follow])
	out := matted{front: sides[0], back: sides[1]}
	out.turnDeg[follow] = reg.deg
	return out, nil
}

// Result of assembling one card from its two frames.
type card struct {
	front, back *image.NRGBA
	size        types.Size
	tilt        [2]float64 // degrees, front then back
	flags       []string
	rescan      bool // a side lost a corner past the tolerance; only flags, tilt and rawFront are set
	rawFront    image.Image
}

func (f *frame) image() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, f.w, f.h))
	for i := 0; i < f.w*f.h; i++ {
		img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = f.pix[3*i], f.pix[3*i+1], f.pix[3*i+2], 255
	}
	return img
}

func buildCard(front, back *frame, threshold float64) (*card, error) {
	frames := [2]*frame{front, back}
	var sigs, lums [2]*image.Gray
	var geos [2]geometry
	c := &card{}
	for i, f := range frames {
		sigs[i], lums[i] = newSignals(f)
		geos[i] = measure(sigs[i], threshold)
		c.tilt[i] = geos[i].theta * 180 / math.Pi
		if geos[i].missing {
			c.flags = append(c.flags, fmt.Sprintf("no edge found on one side of the %s", [2]string{"front", "back"}[i]))
		}
		if geos[i].onOutline < minOnOutline {
			c.flags = append(c.flags, fmt.Sprintf("only %.0f%% of the %s's edge is on one outline: a double-feed?", geos[i].onOutline*100, [2]string{"front", "back"}[i]))
		}
		if lost := geos[i].clippedBy(sigs[i]); lost > clipToleranceCm*pxPerCm {
			c.rescan = true
			c.flags = append(c.flags, fmt.Sprintf("%s: a corner was clipped by about %.1f mm: keep the card snug against the guides, straightest edge leading",
				[2]string{"front", "back"}[i], lost/pxPerCm*10))
		}
	}
	if math.Abs(geos[0].width()-geos[1].width()) > maxSideDifference*pxPerCm ||
		math.Abs(geos[0].height()-geos[1].height()) > maxSideDifference*pxPerCm {
		c.flags = append(c.flags, "the two sides measure different sizes: are they the same card?")
	}

	if c.rescan {
		c.rawFront = front.image()
		return c, nil
	}

	cv := makeCanvases(frames, sigs, lums, geos)
	m, err := cv.matte()
	if err != nil {
		return nil, err
	}
	c.flags = append(c.flags, m.flags...)
	c.tilt[0] += m.turnDeg[0]
	c.tilt[1] += m.turnDeg[1]
	sides, err := trimTogether(m.front, m.back)
	if err != nil {
		return nil, err
	}
	c.front, c.back = sides[0], sides[1]

	c.size = types.Size{PxWidth: c.front.Bounds().Dx(), PxHeight: c.front.Bounds().Dy()}
	xRes := new(big.Rat).SetFloat64(pxPerCm)
	yRes := new(big.Rat).SetFloat64(pxPerCm)
	c.size.SetResolution(xRes, yRes)
	return c, nil
}

// trimTogether crops both sides with one box, the front's, so the back's crop is its mirror
// image and the sides still map onto each other. The box is the union of the front's visible
// pixels and the back's, mirrored.
func trimTogether(front, back *image.NRGBA) ([2]*image.NRGBA, error) {
	var out [2]*image.NRGBA
	fb, bb := visibleBounds(front), visibleBounds(back)
	if fb.Empty() || bb.Empty() {
		return out, errors.New("no card found in the scan")
	}
	mirror := func(r image.Rectangle) image.Rectangle {
		return image.Rect(front.Bounds().Dx()-r.Max.X, r.Min.Y, front.Bounds().Dx()-r.Min.X, r.Max.Y)
	}
	box := fb.Union(mirror(bb))
	for i, c := range []struct {
		src *image.NRGBA
		box image.Rectangle
	}{{front, box}, {back, mirror(box)}} {
		dst := image.NewNRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
		for y := 0; y < box.Dy(); y++ {
			copy(dst.Pix[y*dst.Stride:][:4*box.Dx()], c.src.Pix[(c.box.Min.Y+y)*c.src.Stride+4*c.box.Min.X:])
		}
		out[i] = dst
	}
	return out, nil
}

func visibleBounds(img *image.NRGBA) image.Rectangle {
	b := img.Bounds()
	r := image.Rectangle{Min: b.Max, Max: b.Min}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.Pix[y*img.Stride+4*x+3] >= trimAlpha {
				r.Min.X, r.Min.Y = min(r.Min.X, x), min(r.Min.Y, y)
				r.Max.X, r.Max.Y = max(r.Max.X, x+1), max(r.Max.Y, y+1)
			}
		}
	}
	if r.Min.X > r.Max.X {
		return image.Rectangle{}
	}
	return r
}
