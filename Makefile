GO_SOURCES := $(shell find pkg formats types internal -name '*.go')

MACOS_MIN := 14.0
IOS_MIN := 17.0

.PHONY: xcframework clean
xcframework: build/Postcards.xcframework

# Builds the Postcards.xcframework used by the SwiftUI app (see
# /Users/jp/src/personal/postcard-collector-app). Needs Xcode's command line
# tools, and (on first run) `go run golang.org/x/mobile/cmd/gomobile init`.
build/Postcards.xcframework: $(GO_SOURCES) go.mod go.sum
	MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN) IPHONEOS_DEPLOYMENT_TARGET=$(IOS_MIN) \
	go run golang.org/x/mobile/cmd/gomobile bind \
	  -target ios,iossimulator,macos \
	  -iosversion $(IOS_MIN) -macosversion $(MACOS_MIN) \
	  -tags "mobile_sqlite sqlite_fts5" -ldflags "-s -w" \
	  -o build/Postcards.xcframework ./pkg/appcore

clean:
	rm -rf build
