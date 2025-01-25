package www

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:generate sh -c "cp \"$(go env GOMODCACHE)/$(go list -m github.com/nlepage/go-wasm-http-server/v2 | tr ' ' '@')/sw.js\" postoffice/sw.js"
//go:generate sh -c "cp \"$(tinygo env TINYGOROOT)/targets/wasm_exec.js\" postoffice/wasm_exec.js"

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
