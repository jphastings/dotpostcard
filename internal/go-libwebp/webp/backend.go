package webp

import (
	"unsafe"
)

type backendFuncs struct {
	Encode encodeFunc
	Free   freeFunc
}

type encodeFunc func(
	in unsafe.Pointer,
	w int32,
	h int32,
	bps int32,
	q float32,
	out unsafe.Pointer,
) uint64

type freeFunc func(unsafe.Pointer)
