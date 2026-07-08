//go:build wasm
// +build wasm

package web

var DefaultCodec, _ = Codec("jpeg", "png")
var PostcardCodec, _ = SingleExtCodec("jpeg", "png")
