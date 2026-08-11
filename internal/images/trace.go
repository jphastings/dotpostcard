package images

import (
	"container/heap"
	"image"
	"math"
)

type ipoint struct{ x, y int }

// toAlpha extracts the image's alpha channel as 8-bit, rebased to (0,0).
func toAlpha(im image.Image) *image.Alpha {
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewAlpha(image.Rect(0, 0, w, h))

	switch src := im.(type) {
	case *image.NRGBA:
		copyAlphaPlane(out, src.Pix, src.PixOffset(b.Min.X, b.Min.Y), src.Stride, w, h)
	case *image.RGBA:
		copyAlphaPlane(out, src.Pix, src.PixOffset(b.Min.X, b.Min.Y), src.Stride, w, h)
	default:
		for y := 0; y < h; y++ {
			row := y * out.Stride
			for x := 0; x < w; x++ {
				_, _, _, a := im.At(b.Min.X+x, b.Min.Y+y).RGBA()
				out.Pix[row+x] = uint8(a >> 8)
			}
		}
	}
	return out
}

func copyAlphaPlane(out *image.Alpha, pix []uint8, offset, stride, w, h int) {
	for y := 0; y < h; y++ {
		row := pix[offset+y*stride:]
		outRow := y * out.Stride
		for x := 0; x < w; x++ {
			out.Pix[outRow+x] = row[x*4+3]
		}
	}
}

// Crack-following contour tracer. Contours are traced along the boundaries
// ("cracks") between opaque and transparent pixels on the pixel-corner
// lattice, keeping opaque pixels on the right of the direction of travel.
// Vertices are integer corner coordinates in [0..w]×[0..h], so a fully
// opaque image traces to exactly its four corners. Foreground is treated as
// 4-connected, which guarantees each contour is a simple polygon.

const (
	dirE = iota
	dirS
	dirW
	dirN
)

var (
	dirVec = [4]ipoint{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	// Pixel flanking a crack from vertex v in direction d, on the left/right
	// of the direction of travel, as offsets from v.
	leftPx  = [4]ipoint{{0, -1}, {0, 0}, {-1, 0}, {-1, -1}}
	rightPx = [4]ipoint{{0, 0}, {-1, 0}, {-1, -1}, {0, -1}}
)

// traceLargestContour returns the corner vertices of the contour enclosing
// the largest area of pixels with alpha >= threshold, or nil if there are
// none. Smaller detached regions (dust specks) and holes lose the area
// contest; holes inside the winning contour are intentionally ignored, as
// postcard silhouettes are simply connected.
func traceLargestContour(alpha *image.Alpha, threshold uint8) []ipoint {
	w, h := alpha.Rect.Dx(), alpha.Rect.Dy()

	filled := make([]bool, w*h)
	for y := 0; y < h; y++ {
		row := y * alpha.Stride
		for x := 0; x < w; x++ {
			filled[y*w+x] = alpha.Pix[row+x] >= threshold
		}
	}
	f := func(x, y int) bool { return x >= 0 && x < w && y >= 0 && y < h && filled[y*w+x] }

	// A contour is entered via a "top crack": the upper edge of an opaque
	// pixel with a transparent pixel above. Each eastward traversal is
	// recorded so every contour is traced exactly once.
	visited := make([]bool, w*h)

	var best []ipoint
	var bestArea int64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if filled[y*w+x] && !f(x, y-1) && !visited[y*w+x] {
				pts := traceFrom(x, y, f, visited, w)
				if a := abs64(shoelace2(pts)); a > bestArea {
					best, bestArea = pts, a
				}
			}
		}
	}
	return best
}

// traceFrom walks the contour through the top crack of pixel (sx,sy),
// returning its corner vertices.
func traceFrom(sx, sy int, f func(int, int) bool, visited []bool, w int) []ipoint {
	v := ipoint{sx, sy}
	d := dirE
	pts := []ipoint{v}

	for {
		if d == dirE {
			visited[v.y*w+v.x] = true
		}
		v = ipoint{v.x + dirVec[d].x, v.y + dirVec[d].y}

		var next int
		switch {
		case !f(v.x+rightPx[d].x, v.y+rightPx[d].y): // wall ends: turn right
			next = (d + 1) % 4
		case !f(v.x+leftPx[d].x, v.y+leftPx[d].y): // wall continues: straight on
			next = d
		default: // concave corner: turn left
			next = (d + 3) % 4
		}

		if v.x == sx && v.y == sy && next == dirE { // back at the start crack
			return pts
		}
		if next != d {
			pts = append(pts, v)
		}
		d = next
	}
}

// shoelace2 returns twice the signed area of the polygon (exact, integer).
func shoelace2(pts []ipoint) int64 {
	var sum int64
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		sum += int64(q.x-p.x) * int64(q.y+p.y)
	}
	return sum
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// Ramer–Douglas–Peucker simplification of a closed loop, in pixel space so
// the tolerance is isotropic. The loop is split at two well-separated
// anchors (rather than at the arbitrary trace start) so the wrap-around
// segment is simplified like any other.
func simplifyClosed(pts []ipoint, epsilon float64) []ipoint {
	if len(pts) <= 4 {
		return pts
	}

	start := 0
	for i, p := range pts {
		if p.x+p.y < pts[start].x+pts[start].y {
			start = i
		}
	}

	rotated := make([]ipoint, 0, len(pts)+1)
	rotated = append(rotated, pts[start:]...)
	rotated = append(rotated, pts[:start]...)

	far := 0
	var farDist float64
	for i, p := range rotated {
		dx, dy := float64(p.x-rotated[0].x), float64(p.y-rotated[0].y)
		if d := dx*dx + dy*dy; d > farDist {
			far, farDist = i, d
		}
	}

	firstHalf := rdpSimplify(rotated[:far+1], epsilon)
	secondHalf := rdpSimplify(append(rotated[far:], rotated[0]), epsilon)

	out := make([]ipoint, 0, len(firstHalf)+len(secondHalf)-2)
	out = append(out, firstHalf...)
	out = append(out, secondHalf[1:len(secondHalf)-1]...)
	return out
}

func rdpSimplify(pts []ipoint, epsilon float64) []ipoint {
	if len(pts) < 3 {
		return pts
	}

	a, b := pts[0], pts[len(pts)-1]
	maxIdx, maxDist := 0, 0.0
	for i := 1; i < len(pts)-1; i++ {
		if d := perpDistance(pts[i], a, b); d > maxDist {
			maxIdx, maxDist = i, d
		}
	}

	if maxDist <= epsilon {
		return []ipoint{a, b}
	}

	left := rdpSimplify(pts[:maxIdx+1], epsilon)
	right := rdpSimplify(pts[maxIdx:], epsilon)
	return append(left[:len(left)-1], right...)
}

func perpDistance(p, a, b ipoint) float64 {
	dx, dy := float64(b.x-a.x), float64(b.y-a.y)
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return math.Hypot(float64(p.x-a.x), float64(p.y-a.y))
	}
	return math.Abs(dx*float64(p.y-a.y)-dy*float64(p.x-a.x)) / math.Sqrt(lenSq)
}

// triangleArea2 returns twice the unsigned area of the triangle (a,b,c),
// exact in integer pixel coordinates.
func triangleArea2(a, b, c ipoint) int64 {
	return abs64(int64(b.x-a.x)*int64(c.y-a.y) - int64(c.x-a.x)*int64(b.y-a.y))
}

// vwItem is a candidate vertex removal, ranked by its effective (triangle)
// area within a min-heap.
type vwItem struct {
	idx     int
	area    int64
	version int
}

type vwHeap []vwItem

func (h vwHeap) Len() int           { return len(h) }
func (h vwHeap) Less(i, j int) bool { return h[i].area < h[j].area }
func (h vwHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *vwHeap) Push(x any) {
	*h = append(*h, x.(vwItem))
}

func (h *vwHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// simplifyToBudget reduces a closed ring to at most maxPoints vertices using
// Visvalingam-Whyatt: the vertex contributing the least triangle area to its
// two neighbours is repeatedly dropped, so the surviving points are those
// that most affect the traced shape. The ring never shrinks below 3 points.
func simplifyToBudget(pts []ipoint, maxPoints int) []ipoint {
	n := len(pts)
	if maxPoints <= 0 || n <= maxPoints || n <= 3 {
		return pts
	}
	target := maxPoints
	if target < 3 {
		target = 3
	}

	prev := make([]int, n)
	next := make([]int, n)
	area := make([]int64, n)
	version := make([]int, n)
	alive := make([]bool, n)
	for i := range pts {
		prev[i] = (i - 1 + n) % n
		next[i] = (i + 1) % n
		alive[i] = true
	}
	for i := range pts {
		area[i] = triangleArea2(pts[prev[i]], pts[i], pts[next[i]])
	}

	h := make(vwHeap, n)
	for i := range pts {
		h[i] = vwItem{idx: i, area: area[i]}
	}
	heap.Init(&h)

	count := n
	for count > target && h.Len() > 0 {
		item := heap.Pop(&h).(vwItem)
		i := item.idx
		// Neighbours change as vertices are removed, so a popped entry may
		// no longer reflect the vertex's current area; skip stale ones.
		if !alive[i] || item.version != version[i] {
			continue
		}
		alive[i] = false
		count--

		p, q := prev[i], next[i]
		next[p], prev[q] = q, p

		removedArea := area[i]
		for _, j := range [2]int{p, q} {
			newArea := triangleArea2(pts[prev[j]], pts[j], pts[next[j]])
			// Monotonicity fix: never let a surviving vertex look less
			// significant than one already removed, or later removals
			// could reorder which detail gets discarded first.
			if newArea < removedArea {
				newArea = removedArea
			}
			area[j] = newArea
			version[j]++
			heap.Push(&h, vwItem{idx: j, area: newArea, version: version[j]})
		}
	}

	out := make([]ipoint, 0, count)
	for i, p := range pts {
		if alive[i] {
			out = append(out, p)
		}
	}
	return out
}
