package matting

// guidedAlpha runs a colour-guided filter (He, Sun & Tang) over p using the
// RGB guidance planes I, returning a soft alpha estimate that follows local
// image structure — paper fibres with any contrast against the backboard
// pull the alpha to their shape. Where guidance has no contrast the result
// degrades to a plain feathering of p, never deleting detail.
func guidedAlpha(I [3]*plane, p *plane, r int, eps float32) *plane {
	n := len(p.v)

	var meanI [3]*plane
	for c := 0; c < 3; c++ {
		meanI[c] = box(I[c], r)
	}
	meanP := box(p, r)

	// Per-pixel 3×3 covariance of the guidance (6 unique elements) and
	// guidance–input covariance.
	var cov [3][3]*plane
	for c1 := 0; c1 < 3; c1++ {
		for c2 := c1; c2 < 3; c2++ {
			corr := box(I[c1].mul(I[c2]), r)
			for i := 0; i < n; i++ {
				corr.v[i] -= meanI[c1].v[i] * meanI[c2].v[i]
			}
			cov[c1][c2] = corr
		}
	}
	var covIp [3]*plane
	for c := 0; c < 3; c++ {
		cross := box(I[c].mul(p), r)
		for i := 0; i < n; i++ {
			cross.v[i] -= meanI[c].v[i] * meanP.v[i]
		}
		covIp[c] = cross
	}

	// Solve (Σ + εI)·a = covIp per pixel via the analytic inverse of the
	// regularised symmetric 3×3 matrix, then b = mean_p − a·mean_I.
	var a [3]*plane
	for c := 0; c < 3; c++ {
		a[c] = newPlane(p.w, p.h)
	}
	b := newPlane(p.w, p.h)
	for i := 0; i < n; i++ {
		s00 := cov[0][0].v[i] + eps
		s11 := cov[1][1].v[i] + eps
		s22 := cov[2][2].v[i] + eps
		s01 := cov[0][1].v[i]
		s02 := cov[0][2].v[i]
		s12 := cov[1][2].v[i]

		c00 := s11*s22 - s12*s12
		c01 := s02*s12 - s01*s22
		c02 := s01*s12 - s02*s11
		c11 := s00*s22 - s02*s02
		c12 := s01*s02 - s00*s12
		c22 := s00*s11 - s01*s01

		det := s00*c00 + s01*c01 + s02*c02
		if det == 0 {
			continue // a stays 0; q falls back to mean_p via b
		}

		v0, v1, v2 := covIp[0].v[i], covIp[1].v[i], covIp[2].v[i]
		a[0].v[i] = (c00*v0 + c01*v1 + c02*v2) / det
		a[1].v[i] = (c01*v0 + c11*v1 + c12*v2) / det
		a[2].v[i] = (c02*v0 + c12*v1 + c22*v2) / det
	}
	for i := 0; i < n; i++ {
		b.v[i] = meanP.v[i] - a[0].v[i]*meanI[0].v[i] - a[1].v[i]*meanI[1].v[i] - a[2].v[i]*meanI[2].v[i]
	}

	for c := 0; c < 3; c++ {
		a[c] = box(a[c], r)
	}
	b = box(b, r)

	q := newPlane(p.w, p.h)
	for i := 0; i < n; i++ {
		v := a[0].v[i]*I[0].v[i] + a[1].v[i]*I[1].v[i] + a[2].v[i]*I[2].v[i] + b.v[i]
		q.v[i] = min(1, max(0, v))
	}
	return q
}
