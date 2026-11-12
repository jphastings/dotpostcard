//go:build wasm

package main

// --llm needs a model backend that doesn't build for wasm.
func registerLLMFlags() {}
