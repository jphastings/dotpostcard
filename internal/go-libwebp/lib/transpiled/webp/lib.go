package webp

import (
	"image"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/lib/common"
	"modernc.org/libc"
)

func DecodeImpl(buf []byte) (image.Image, error) {
	disableVP8GetCPUInfo()
	tls := libc.NewTLS()
	defer tls.Close()
	return common.Decode(buf, makeDecodeImpl(tls), makeFreeImpl(tls))
}

func EncodeImpl(w io.Writer, m *image.NRGBA, quality float32) error {
	tls := libc.NewTLS()
	defer tls.Close()
	return common.Encode(w, m, quality, makeEncodeImpl(tls), makeFreeImpl(tls))
}

func makeDecodeImpl(tls *libc.TLS) common.DecoderFunc {
	return func(data uintptr, data_size uint64, width, height uintptr) uintptr {
		return WebPDecodeRGBA(tls, data, data_size, width, height)
	}
}

func makeEncodeImpl(tls *libc.TLS) common.EncodeFunc {
	return func(in uintptr, w, h, bps int32, q float32, out uintptr) uint64 {
		if q >= 1.0 {
			return WebPEncodeLosslessRGBA(tls, in, w, h, bps, out)
		}
		return WebPEncodeRGBA(tls, in, w, h, bps, q*100, out)
	}
}

func makeFreeImpl(tls *libc.TLS) common.FreeFunc {
	return func(p uintptr) {
		WebPFree(tls, p)
	}
}

func disableVP8GetCPUInfo() {
	// HACK: Avoid panic when decoding lossy webp data.
	//
	// Inline asm is used for CPU feature detection; ccgo doesn't support inline asm.
	//
	// libwebp uses this function pointer to check for any specialized CPU instructions.
	// If not nil (which it is for well supported platforms, like amd64, and arm64), it will try
	// to check CPU features using inline asm. ccgo inserts a panic for inline asm, thus using
	// this will panic.
	//
	// To avoid this terrible fate, we must manually turn off the CPU feature detection by
	// nil'ing out this function pointer. This must be done after package init since that is
	// is when this value is set by the transpiled code.
	VP8GetCPUInfo = 0
}
