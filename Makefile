GO_SOURCES := $(shell find pkg formats types internal -name '*.go')

MACOS_MIN := 14.0
IOS_MIN := 17.0

.PHONY: xcframework clean
xcframework: build/Postcards.xcframework

# Builds the Postcards.xcframework used by the SwiftUI app (see
# /Users/jp/src/personal/postcard-collector-app). Needs Xcode's command line
# tools, and (on first run) `go run golang.org/x/mobile/cmd/gomobile init`.
#
# Pin GOTOOLCHAIN to 1.26.x: go 1.25.0 miscompiles this gomobile-bound library,
# corrupting the heap at runtime (crashes surfaced only in release builds, never
# under a local 1.26 toolchain). Scoped here rather than via a go.mod `toolchain`
# directive so TinyGo — which builds the WASM serviceworker and supports only up
# to go 1.25 — keeps using the `go 1.25.0` directive elsewhere.
build/Postcards.xcframework: $(GO_SOURCES) go.mod go.sum
	MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN) IPHONEOS_DEPLOYMENT_TARGET=$(IOS_MIN) \
	GOTOOLCHAIN=go1.26.4 \
	go run golang.org/x/mobile/cmd/gomobile bind \
	  -target ios,iossimulator,macos \
	  -iosversion $(IOS_MIN) -macosversion $(MACOS_MIN) \
	  -tags "mobile_sqlite sqlite_fts5" -ldflags "-s -w" \
	  -o build/Postcards.xcframework ./pkg/appcore

clean:
	rm -rf build
