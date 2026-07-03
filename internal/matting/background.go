package matting

// backgroundField estimates the backboard colour at every pixel by
// normalized convolution over known-background pixels: a local mean that
// follows lid shading without assuming any global backboard colour. Pixels
// with no background in reach fall back to the global background mean.
func backgroundField(I [3]*plane, bgMask *plane, r int, fallback [3]float32) [3]*plane {
	den := box(bgMask, r)

	var B [3]*plane
	for c := 0; c < 3; c++ {
		num := box(I[c].mul(bgMask), r)
		B[c] = newPlane(bgMask.w, bgMask.h)
		for i := range num.v {
			if den.v[i] > 1e-6 {
				B[c].v[i] = num.v[i] / den.v[i]
			} else {
				B[c].v[i] = fallback[c]
			}
		}
	}
	return B
}
