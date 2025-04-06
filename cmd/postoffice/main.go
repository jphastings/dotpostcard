package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jphastings/dotpostcard/internal/www"
	"github.com/jphastings/dotpostcard/pkg/postoffice"
)

func main() {
	codecChoices, err := postoffice.DefaultCodecChoices()
	check(err, "Unable to load codecs")

	http.Handle("/", www.PostOfficeHandler)
	http.HandleFunc("/api/compile/", postoffice.CompileFromForm(codecChoices))

	port, gavePort := os.LookupEnv("PORT")
	if !gavePort {
		port = "7678"
	}

	// This can be accessed on any IP address, but ServiceWorkers will only function in
	// a secure context (localhost, or over HTTPS).
	fmt.Printf("Starting server. Access at http://127.0.0.1:%s\n", port)
	check(http.ListenAndServe(":"+port, nil), "Error starting server")
}

func check(err error, message string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
		os.Exit(1)
	}
}
