package main

import (
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto/pdsbundle"
)

// atprotoFormatName is handled specially here, the way formats.go handles supportFiles: it's
// not a registered codec, so it's stripped from the format list before CodecsByFormat sees it.
const atprotoFormatName = "atproto"

// stripATProtoFormat removes "atproto" from names, reporting whether it was present.
func stripATProtoFormat(names []string) ([]string, bool) {
	var rest []string
	wanted := false
	for _, name := range names {
		if name == atprotoFormatName {
			wanted = true
			continue
		}
		rest = append(rest, name)
	}
	return rest, wanted
}

// formatCount reports how many formats a run is converting into, for the "Converting…"
// progress line: atproto isn't a registered codec, so it isn't in codecCount already.
func formatCount(codecCount int, uploadToATProto bool) int {
	if uploadToATProto {
		return codecCount + 1
	}
	return codecCount
}

// partitionInputs splits paths into local filesystem paths and at:// record URIs.
func partitionInputs(paths []string) (local, atURIs []string) {
	for _, p := range paths {
		if strings.HasPrefix(p, "at://") {
			atURIs = append(atURIs, p)
		} else {
			local = append(local, p)
		}
	}
	return local, atURIs
}

// atBundlesFromURIs resolves each at:// URI into a readable bundle. A URI that fails to parse
// or resolve is reported through warn rather than aborting the run.
func atBundlesFromURIs(uris []string, pdsHostOverride, plcHostOverride string, warn func(uri, msg string)) []formats.Bundle {
	var bundles []formats.Bundle
	for _, uri := range uris {
		bundle, err := pdsbundle.NewBundle(uri, pdsHostOverride, plcHostOverride, func(msg string) { warn(uri, msg) })
		if err != nil {
			warn(uri, err.Error())
			continue
		}
		bundles = append(bundles, bundle)
	}
	return bundles
}
