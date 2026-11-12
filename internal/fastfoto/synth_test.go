package fastfoto

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
)

// card is drawn at the scanner's resolution, so sizes in these tests are real.
type synth struct {
	w, h        int
	widthCm     float64
	heightCm    float64
	tiltDeg     float64
	paper       color.RGBA
	zigzagPx    float64 // tooth depth on the bottom edge
	shadow      bool    // dark core then a soft ramp just beyond the bottom edge
	doubleFeed  bool    // a second card shingled half a card away
	sideShadow  bool    // soft ramp, darkest at the edge, beyond the left and right edges
	chromaNoise int     // extra colour noise in pairs of pixels, as the scanner's JPEG makes
	bandPx      [2]int  // width of a hard dark band just beyond the left and right edges
	frameBandPx int     // dark rows along the frame's very top and bottom
	edgeDots    bool    // neutral grey halftone printed up to the top edge: darker than the backing, but warmer
	redShadow   bool    // backing beside the left and right edges a little darker, but redder
}

const sideShadowPx = 100

// croppedSynth is a card in a frame only as big as its bounding box plus marginPx of
// backing on every side, as the scanner's hardware crop delivers it.
func croppedSynth(widthCm, heightCm, tiltDeg float64, marginPx int) synth {
	s := defaultSynth()
	s.widthCm, s.heightCm, s.tiltDeg, s.paper = widthCm, heightCm, tiltDeg, color.RGBA{215, 175, 140, 255}
	sin, cos := math.Sincos(math.Abs(tiltDeg) * math.Pi / 180)
	w, h := widthCm*pxPerCm, heightCm*pxPerCm
	s.w, s.h = int(math.Ceil(w*cos+h*sin))+2*marginPx, int(math.Ceil(w*sin+h*cos))+2*marginPx
	return s
}

func defaultSynth() synth {
	return synth{w: 1700, h: 920, widthCm: 5, heightCm: 3.4, paper: color.RGBA{242, 239, 236, 255}}
}

var (
	backingColour = color.RGBA{165, 173, 179, 255}
	bandColour    = color.RGBA{60, 60, 60, 255}
)

// noise is a deterministic ±4 texture.
func noise(x, y int) int {
	n := uint32(x)*73856093 ^ uint32(y)*19349663
	n ^= n >> 13
	n *= 0x5bd1e995
	return int(n>>24)%9 - 4
}

func (s synth) insideCard(u, v, offU, offV float64) (inside bool, bottomEdge float64) {
	hw, hh := s.widthCm*pxPerCm/2, s.heightCm*pxPerCm/2
	u, v = u-offU, v-offV
	limit := hh
	if s.zigzagPx > 0 {
		phase := math.Mod(math.Abs(u), 60) / 60
		limit = hh - s.zigzagPx*math.Abs(2*phase-1)
	}
	return math.Abs(u) <= hw && v >= -hh && v <= limit, limit
}

func (s synth) image() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, s.w, s.h))
	sin, cos := math.Sincos(s.tiltDeg * math.Pi / 180)
	cx, cy := float64(s.w)/2, float64(s.h)/2
	hh := s.heightCm * pxPerCm / 2
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			u, v := dx*cos+dy*sin, -dx*sin+dy*cos
			inside, bottom := s.insideCard(u, v, 0, 0)
			if s.doubleFeed {
				second, _ := s.insideCard(u, v, s.widthCm*pxPerCm/2, hh)
				inside = inside || second
			}
			c := backingColour
			switch {
			case inside:
				c = s.paper
			case s.shadow && math.Abs(u) <= s.widthCm*pxPerCm/2 && v > bottom && v <= bottom+70:
				t := math.Max(0, (v-bottom-10)/60)
				c = color.RGBA{uint8(60 + t*(float64(c.R)-60)), uint8(60 + t*(float64(c.G)-60)), uint8(60 + t*(float64(c.B)-60)), 255}
			}
			if beyond := math.Abs(u) - s.widthCm*pxPerCm/2; !inside && math.Abs(v) <= hh && beyond > 0 && beyond <= float64(s.bandPx[min(1, max(0, int(math.Copysign(1, u)+1)/2))]) {
				c = bandColour
			}
			if s.sideShadow && math.Abs(v) <= hh {
				if beyond := math.Abs(u) - s.widthCm*pxPerCm/2; beyond > 0 && beyond <= sideShadowPx {
					dark := 45 * (1 - beyond/sideShadowPx)
					c = color.RGBA{uint8(float64(c.R) - dark), uint8(float64(c.G) - dark), uint8(float64(c.B) - dark), 255}
				}
			}
			hw := s.widthCm * pxPerCm / 2
			if s.edgeDots && inside && v < -hh+30 && math.Mod(u+hw, 6) < 5 && math.Mod(v+hh, 6) < 5 {
				c = color.RGBA{160, 160, 160, 255}
			}
			if s.redShadow && math.Abs(v) <= hh {
				if beyond := math.Abs(u) - s.widthCm*pxPerCm/2; beyond > 0 && beyond <= sideShadowPx {
					c = color.RGBA{c.R + 3, c.G - 8, c.B - 2, 255}
				}
			}
			if y < s.frameBandPx || y >= s.h-s.frameBandPx {
				c = bandColour
			}
			n := noise(x, y)
			chroma := s.chromaNoise * noise(x/2, y) / 4
			img.SetRGBA(x, y, color.RGBA{clamp8(int(c.R) + n + chroma), clamp8(int(c.G) + n), clamp8(int(c.B) + n - chroma), 255})
		}
	}
	return img
}

func clamp8(v int) uint8 { return uint8(min(255, max(0, v))) }

// jpegBytes encodes the frame, then claims the scanner's maximum height in the SOF
// header like the real thing does.
func (s synth) jpegBytes() []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, s.image(), &jpeg.Options{Quality: 95}); err != nil {
		panic(err)
	}
	b := buf.Bytes()
	// A comment segment carries the true height for the fake scanner to report.
	com := []byte(fmt.Sprintf("h=%d", s.h))
	b = append(append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, byte(len(com) + 2)}, com...), b[2:]...)
	for i := 2; i+9 < len(b); {
		if b[i+1] == 0xC0 {
			b[i+5], b[i+6] = 9283>>8, 9283&0xFF
			break
		}
		i += 2 + int(b[i+2])<<8 + int(b[i+3])
	}
	return b
}
