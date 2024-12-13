package www

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed postoffice/*
var postOffice embed.FS

var PostOfficeHandler http.Handler

func init() {
	ff, err := fs.Sub(postOffice, "postoffice")
	if err != nil {
		panic(err)
	}

	PostOfficeHandler = http.FileServer(http.FS(ff))
}
