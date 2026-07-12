package matting

// plane is a single-channel float image over a w×h grid.
type plane struct {
	w, h int
	v    []float32
}

func newPlane(w, h int) *plane {
	return &plane{w: w, h: h, v: make([]float32, w*h)}
}

func (p *plane) mul(q *plane) *plane {
	out := newPlane(p.w, p.h)
	for i, v := range p.v {
		out.v[i] = v * q.v[i]
	}
	return out
}

// box replaces each value with its mean over a (2r+1)² window via an
// integral image, so cost is independent of radius. Windows are clamped at
// the plane's edges and normalized by their actual size, avoiding dark rims.
func box(src *plane, r int) *plane {
	w, h := src.w, src.h
	sat := make([]float64, (w+1)*(h+1))
	for y := 0; y < h; y++ {
		rowSum := 0.0
		satRow := (y + 1) * (w + 1)
		prevRow := y * (w + 1)
		for x := 0; x < w; x++ {
			rowSum += float64(src.v[y*w+x])
			sat[satRow+x+1] = sat[prevRow+x+1] + rowSum
		}
	}

	out := newPlane(w, h)
	for y := 0; y < h; y++ {
		y0, y1 := max(0, y-r), min(h-1, y+r)
		for x := 0; x < w; x++ {
			x0, x1 := max(0, x-r), min(w-1, x+r)
			sum := sat[(y1+1)*(w+1)+x1+1] - sat[y0*(w+1)+x1+1] -
				sat[(y1+1)*(w+1)+x0] + sat[y0*(w+1)+x0]
			out.v[y*w+x] = float32(sum / float64((y1-y0+1)*(x1-x0+1)))
		}
	}
	return out
}
