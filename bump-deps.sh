# Get latest Go dependencies
go get -u ./...

# Retrieve the correct Service Worker & WASM exec files for the postoffice-serviceworker
curl -o internal/www/postoffice/wasm_exec.js "https://cdn.jsdelivr.net/gh/golang/go@$(go version | cut -f3 -d' ')/misc/wasm/wasm_exec.js"
curl -o internal/www/postoffice/sw.js "https://cdn.jsdelivr.net/gh/nlepage/go-wasm-http-server@$(go list -m github.com/nlepage/go-wasm-http-server/v2 | cut -f2 -d' ')/sw.js"
