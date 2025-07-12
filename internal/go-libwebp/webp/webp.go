// Package webp is an idiomatic Go wrapper for libwebp codec.
package webp

import (
	"image"
)

func init() {
	image.RegisterFormat("webp", "RIFF????WEBP", Decode, DecodeConfig)
}
