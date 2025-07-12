// Package common captures the unsafe orchestration that needs to occur when interacting
// with libwebp. The function implementations can be swapped out based on the desired
// implementation.
package common

import (
	"fmt"
	"image"
	"io"
	"runtime"
	"unsafe"
)

type EncodeFunc func(in uintptr, w int32, h int32, bps int32, q float32, out uintptr) (size uint64)

type DecoderFunc func(data uintptr, data_size uint64, width uintptr, height uintptr) (rgba uintptr)

type FreeFunc func(uintptr)

// Encode an RGBA webp image into the provided writer.
func Encode(w io.Writer, m *image.NRGBA, q float32, enc EncodeFunc, free FreeFunc) error {
	p := runtime.Pinner{}
	defer p.Unpin()

	out := new(*byte)
	pix := unsafe.SliceData(m.Pix)

	p.Pin(pix)
	p.Pin(out)

	size := enc(
		uintptr(unsafe.Pointer(pix)),
		int32(m.Bounds().Dx()),
		int32(m.Bounds().Dy()),
		int32(m.Stride),
		q,
		uintptr(unsafe.Pointer(out)),
	)

	if size == 0 {
		return fmt.Errorf("empty result")
	}

	if *out == nil {
		return fmt.Errorf("failed to allocate output buffer")
	}

	defer free(uintptr(unsafe.Pointer(*out)))

	buf := unsafe.Slice(*out, size)

	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("writing the encoded webp: %w", err)
	}

	return nil
}

// Decode the webp data into an [image.Image].
func Decode(buf []byte, dec DecoderFunc, free FreeFunc) (image.Image, error) {
	p := runtime.Pinner{}
	defer p.Unpin()

	data := unsafe.SliceData(buf)
	width := new(int32)
	height := new(int32)

	p.Pin(data)
	p.Pin(width)
	p.Pin(height)

	samples := (*uint8)(unsafe.Pointer(dec(
		uintptr(unsafe.Pointer(data)),
		uint64(len(buf)),
		uintptr(unsafe.Pointer(width)),
		uintptr(unsafe.Pointer(height)),
	)))

	if samples == nil || width == nil || height == nil {
		return nil, fmt.Errorf("failed decoding webp into rgba")
	}

	defer free(uintptr(unsafe.Pointer(samples)))

	w, h := int(*width), int(*height)

	size := w * h * 4

	pix := make([]uint8, size)

	// PERF: I wonder if we could use a finalizer to free the memory when the image is no
	// longer reachable. That way we could avoid this copy.
	copy(pix, unsafe.Slice(samples, size))

	m := &image.NRGBA{
		Pix:    pix,
		Rect:   image.Rectangle{Max: image.Point{X: w, Y: h}},
		Stride: w * 4,
	}

	return m, nil
}
