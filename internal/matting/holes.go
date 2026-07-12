package matting

import "image"

// fillInteriorHoles restores full opacity to any not-fully-opaque region
// that can't be reached from the image border through other non-opaque
// pixels. Postcards are solid card, so real transparency only enters from
// outside the edge; enclosed dips are matting artifacts (dark print or
// texture near the band) and get their original pixels back.
func fillInteriorHoles(out, src *image.NRGBA) {
	w, h := out.Rect.Dx(), out.Rect.Dy()

	const opaque = 255
	seen := make([]bool, w*h)
	stack := make([]int, 0, 2*(w+h))

	push := func(x, y int) {
		i := y*w + x
		if !seen[i] && out.Pix[i*4+3] < opaque {
			seen[i] = true
			stack = append(stack, i)
		}
	}

	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}

	// 8-connected flood, matching the 4-connected foreground convention of
	// the outline tracer (foreground 4-connected ⇒ background 8-connected)
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := i%w, i/w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := x+dx, y+dy
				if nx >= 0 && nx < w && ny >= 0 && ny < h {
					push(nx, ny)
				}
			}
		}
	}

	for i := 0; i < w*h; i++ {
		if out.Pix[i*4+3] < opaque && !seen[i] {
			copy(out.Pix[i*4:i*4+3], src.Pix[i*4:i*4+3])
			out.Pix[i*4+3] = opaque
		}
	}
}
