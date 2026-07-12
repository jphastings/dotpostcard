package matting

const (
	// Alpha below which a band pixel is considered pure background noise.
	alphaFloor = 0.02
	// Divisor floor preventing extreme colour amplification at tiny alphas.
	minAlphaDivisor = 0.05
)

// decontaminate recovers the true foreground colour of a partially
// transparent pixel by subtracting the backboard's contribution:
// I = α·F + (1−α)·B  ⇒  F = (I − (1−α)·B) / α.
// This is what removes the backboard halo from fibrous edges without
// touching the fibres themselves.
func decontaminate(i, b [3]float32, alpha float32) [3]float32 {
	var f [3]float32
	div := max(alpha, minAlphaDivisor)
	for c := 0; c < 3; c++ {
		f[c] = min(1, max(0, (i[c]-(1-alpha)*b[c])/div))
	}
	return f
}

// levels applies a mild contrast stretch so near-0/near-1 guided-filter
// output snaps to fully transparent/opaque.
func levels(alpha float32) float32 {
	return min(1, max(0, (alpha-0.02)/0.96))
}
