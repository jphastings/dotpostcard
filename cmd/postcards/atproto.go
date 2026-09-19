package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/types"
	"github.com/spf13/cobra"
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

// atCredentials resolves --at-user/--at-password, falling back to the same environment
// variables goat checks, in the same order.
func atCredentials(cmd *cobra.Command) (user, password string, err error) {
	user, _ = cmd.Flags().GetString("at-user")
	password, _ = cmd.Flags().GetString("at-password")

	if user == "" {
		user = firstEnv("GOAT_USERNAME", "ATP_USERNAME", "ATP_AUTH_USERNAME")
	}
	if password == "" {
		password = firstEnv("GOAT_PASSWORD", "ATP_PASSWORD", "ATP_AUTH_PASSWORD")
	}

	if user == "" || password == "" {
		return "", "", fmt.Errorf("uploading to atproto needs credentials: pass --at-user/--at-password, or set GOAT_USERNAME/GOAT_PASSWORD (or ATP_USERNAME/ATP_PASSWORD, or ATP_AUTH_USERNAME/ATP_AUTH_PASSWORD)")
	}

	return user, password, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

func atPDSHostOverride() string { return os.Getenv("ATP_PDS_HOST") }
func atPLCHostOverride() string { return os.Getenv("ATP_PLC_HOST") }

// atBundlesFromURIs resolves each at:// URI into a readable bundle. A URI that fails to parse
// or resolve is reported through warn rather than aborting the run.
func atBundlesFromURIs(uris []string, pdsHostOverride, plcHostOverride string, warn func(uri, msg string)) []formats.Bundle {
	var bundles []formats.Bundle
	for _, uri := range uris {
		bundle, err := atproto.NewBundle(uri, pdsHostOverride, plcHostOverride, func(msg string) { warn(uri, msg) })
		if err != nil {
			warn(uri, err.Error())
			continue
		}
		bundles = append(bundles, bundle)
	}
	return bundles
}

// errRecordExists marks an upload failure caused by the record already existing, so the
// caller can format it like the existing-output-file check does, distinct from other upload
// failures.
var errRecordExists = errors.New("already exists (pass --overwrite to replace it, or --skip-existing to ignore)")

// uploadPostcard encodes pc with the web codec and uploads the resulting image as a blob and
// record. The record's key is only knowable once the image is encoded (it's derived from the
// image's own bytes), so the existing-record check happens here too, right before anything is
// actually uploaded — skip reports that an existing record was left alone because skipExisting
// asked for that, rather than failing the card.
func uploadPostcard(client *atproto.Client, pc types.Postcard, encOpts *formats.EncodeOptions, overwrite, skipExisting bool) (atURI string, skip bool, err error) {
	fws, err := web.DefaultCodec.Encode(pc, encOpts)
	if err != nil {
		return "", false, fmt.Errorf("encoding for upload: %w", err)
	}

	var imageFW *formats.FileWriter
	for i := range fws {
		if strings.HasPrefix(fws[i].Mimetype, "image/") {
			imageFW = &fws[i]
			break
		}
	}
	if imageFW == nil {
		return "", false, fmt.Errorf("the web codec produced no image to upload")
	}

	data, err := imageFW.Bytes()
	if err != nil {
		return "", false, fmt.Errorf("encoding image for upload: %w", err)
	}

	// Encode fills in the image's actual pixel dimensions (and may resize), so the record
	// must be built from what's actually embedded, not pc.Meta as it stood before encoding —
	// otherwise every download would immediately warn about a frontSize mismatch.
	decoded, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), pc.Name).Decode(formats.DecodeOptions{})
	if err != nil {
		return "", false, fmt.Errorf("verifying encoded image: %w", err)
	}

	rkey := atproto.RecordKey(pc.Meta.SentOn, time.Now(), data)
	atURI = fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.RecordType, rkey)

	if !overwrite {
		exists, err := client.RecordExists(rkey)
		if err != nil {
			return "", false, fmt.Errorf("checking atproto for an existing record: %w", err)
		}
		if exists {
			if skipExisting {
				return atURI, true, nil
			}
			return "", false, fmt.Errorf("%s %w", atURI, errRecordExists)
		}
	}

	blob, err := client.UploadBlob(data, imageFW.Mimetype)
	if err != nil {
		return "", false, fmt.Errorf("uploading image: %w", err)
	}

	if err := client.PutRecord(rkey, atproto.FromMetadata(decoded.Meta, blob)); err != nil {
		return "", false, err
	}

	return atURI, false, nil
}
