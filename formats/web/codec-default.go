//go:build !wasm
// +build !wasm

package web

var DefaultCodec, _ = Codec("jpeg", "webp")
var PostcardCodec, _ = SingleExtCodec("jpeg", "webp")
