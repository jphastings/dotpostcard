package matting

// chamferScale is the distance of one orthogonal pixel step in the 3-4
// chamfer metric; diagonal steps cost 4.
const chamferScale = 3

const distInf = int32(1 << 30)

// distanceToBoundary returns, for every pixel, the approximate distance (in
// the 3-4 chamfer metric) to the nearest mask boundary — a pixel whose
// 4-neighbourhood contains both mask values. Boundary pixels have distance 0.
func distanceToBoundary(mask []bool, w, h int) []int32 {
	dist := make([]int32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			dist[i] = distInf
			m := mask[i]
			if (x > 0 && mask[i-1] != m) || (x < w-1 && mask[i+1] != m) ||
				(y > 0 && mask[i-w] != m) || (y < h-1 && mask[i+w] != m) {
				dist[i] = 0
			}
		}
	}

	// Forward pass: top-left to bottom-right
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			d := dist[i]
			if x > 0 {
				d = min(d, dist[i-1]+3)
			}
			if y > 0 {
				d = min(d, dist[i-w]+3)
				if x > 0 {
					d = min(d, dist[i-w-1]+4)
				}
				if x < w-1 {
					d = min(d, dist[i-w+1]+4)
				}
			}
			dist[i] = d
		}
	}

	// Backward pass: bottom-right to top-left
	for y := h - 1; y >= 0; y-- {
		for x := w - 1; x >= 0; x-- {
			i := y*w + x
			d := dist[i]
			if x < w-1 {
				d = min(d, dist[i+1]+3)
			}
			if y < h-1 {
				d = min(d, dist[i+w]+3)
				if x < w-1 {
					d = min(d, dist[i+w+1]+4)
				}
				if x > 0 {
					d = min(d, dist[i+w-1]+4)
				}
			}
			dist[i] = d
		}
	}

	return dist
}
