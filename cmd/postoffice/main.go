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
	http.HandleFunc("/compile/", postoffice.HTTPFormHander(choices))

	port, gavePort := os.LookupEnv("PORT")
	if !gavePort {
		port = "7678"
	}

	fmt.Printf("Starting server on :%s...\n", port)
	check(http.ListenAndServe(":"+port, nil), "Error starting server")
}

func check(err error, message string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
		os.Exit(1)
	}
}
