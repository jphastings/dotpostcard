// Package pdsupload publishes a postcard to an atproto PDS. It lives apart from
// formats/atproto because it needs the web codec's image toolchain.
package pdsupload

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/types"
)

// ErrRecordExists marks an upload failure caused by the record already existing, so the
// caller can format it like the existing-output-file check does, distinct from other upload
// failures.
var ErrRecordExists = errors.New("already exists (pass --overwrite to replace it, or --skip-existing to ignore)")

// UploadPostcard encodes pc with the web codec and uploads the resulting image as a blob and
// record. The record's key is only knowable once the image is encoded (it's derived from the
// image's own bytes), so the existing-record check happens here too, right before anything is
// actually uploaded — skip reports that an existing record was left alone because skipExisting
// asked for that, rather than failing the card.
func UploadPostcard(client *atproto.Client, pc types.Postcard, encOpts *formats.EncodeOptions, overwrite, skipExisting bool) (atURI string, skip bool, err error) {
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
			return "", false, fmt.Errorf("%s %w", atURI, ErrRecordExists)
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
