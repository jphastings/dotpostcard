//go:build js
// +build js

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jphastings/dotpostcard/pkg/postoffice"
	wasmhttp "github.com/nlepage/go-wasm-http-server/v2"
)

func main() {
	codecChoices, err := postoffice.DefaultCodecChoices()
	check(err, "Unable to load codecs")

	http.HandleFunc("/", postoffice.HTTPFormHander(codecChoices))
	wasmhttp.Serve(nil)

	select {}
}

func check(err error, message string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
		os.Exit(1)
	}
}
