#!/bin/bash
cd "$(dirname "$0")"

# This only works on Linux at the moment
echo "Copying WASM & Service Worker exec environment JS into postoffice…"
cp "$(go env GOMODCACHE)/$(go list -m github.com/nlepage/go-wasm-http-server/v2 | tr ' ' '@')/sw.js" internal/www/postoffice/sw.js
cp "$(dirname $(dirname $(which tinygo)))/lib/tinygo/targets/wasm_exec.js" internal/www/postoffice/wasm_exec.js
