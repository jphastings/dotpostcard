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

	if _, err := putWebFile(client, rkey, data, imageFW.Mimetype, decoded.Meta, pc.Name); err != nil {
		return "", false, err
	}

	return atURI, false, nil
}

// putWebFile uploads data as a blob and writes the record for it at rkey, carrying meta with
// its effective title.
func putWebFile(client *atproto.Client, rkey string, data []byte, mimetype string, meta types.Metadata, name string) (cid string, err error) {
	blob, err := client.UploadBlob(data, mimetype)
	if err != nil {
		return "", fmt.Errorf("uploading image: %w", err)
	}

	meta.Title = meta.EffectiveTitle(name)
	return client.PutRecord(rkey, atproto.FromMetadata(meta, blob))
}

// WebFileKey is the record key UploadWebFile would use for data, so a dry run can look for an
// existing record without writing anything.
func WebFileKey(data []byte, name string, fallbackDay time.Time) (string, types.Metadata, error) {
	pc, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), name).Decode(formats.DecodeOptions{})
	if err != nil {
		return "", types.Metadata{}, fmt.Errorf("decoding %s: %w", name, err)
	}
	return atproto.RecordKey(pc.Meta.SentOn, fallbackDay, data), pc.Meta, nil
}

// UploadWebFile uploads an already-encoded web-format postcard (data, of mimetype) untouched,
// as the blob of a record built from the metadata embedded in it and titled by its effective
// title for name. The key derives from data and fallbackDay (used when the card is undated), so
// the same inputs always reach the same record: if one exists there already it is reused, and
// reused is true with its CID returned and nothing written.
func UploadWebFile(client *atproto.Client, data []byte, mimetype, name string, fallbackDay time.Time) (atURI, cid string, reused bool, err error) {
	rkey, meta, err := WebFileKey(data, name, fallbackDay)
	if err != nil {
		return "", "", false, err
	}
	atURI = fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.RecordType, rkey)

	_, existingCID, err := client.GetRecord(client.DID(), rkey)
	if err == nil {
		return atURI, existingCID, true, nil
	}
	if !errors.Is(err, atproto.ErrRecordNotFound) {
		return "", "", false, fmt.Errorf("checking atproto for an existing record: %w", err)
	}

	cid, err = putWebFile(client, rkey, data, mimetype, meta, name)
	if err != nil {
		return "", "", false, err
	}
	return atURI, cid, false, nil
}
