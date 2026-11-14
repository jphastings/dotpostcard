package pdscollection

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdsbundle"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/pkg/collection"
)

// FetchOptions configures Fetch.
type FetchOptions struct {
	// PDSHost and PLCHost override identity resolution, as for atproto.Anonymous.
	PDSHost, PLCHost string
	// Warn, if set, is told about each card whose record and image metadata disagree.
	Warn func(string)
}

var blobExtensions = map[string]string{"image/webp": "webp", "image/jpeg": "jpg", "image/png": "png"}

// Fetch builds a new *.postcards file at path from the published collection at uri, returning
// the names of its cards. Each image is stored exactly as published, named from its record's
// title. It refuses to overwrite an existing file, and removes a partial one on failure.
func Fetch(uri, path string, opts FetchOptions) (names []string, err error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%s already exists", path)
	}

	published, records, err := atproto.FetchCollection(uri, opts.PDSHost, opts.PLCHost)
	if err != nil {
		return nil, err
	}

	coll, err := collection.Create(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		coll.Close()
		if err != nil {
			os.Remove(path)
		}
	}()

	if err := coll.SetTitle(published.Title); err != nil {
		return nil, err
	}

	clients := map[string]*atproto.Client{}
	taken := map[string]bool{}
	for i, record := range records {
		ref := published.Postcards[i]
		authority, _, _, _ := atproto.ParseATURI(ref.URI)
		client, ok := clients[authority]
		if !ok {
			if client, err = atproto.Anonymous(authority, opts.PDSHost, opts.PLCHost); err != nil {
				return nil, err
			}
			clients[authority] = client
		}

		ext, ok := blobExtensions[record.Image.MimeType]
		if !ok {
			return nil, fmt.Errorf("%s: unsupported image type %q", ref.URI, record.Image.MimeType)
		}
		data, err := client.GetBlob(client.DID(), record.Image.Ref.Link)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ref.URI, err)
		}

		baseName := pdsbundle.NameFromRecord(record, ref.CID)
		name := uniqueName(baseName, taken)
		if opts.Warn != nil {
			if err := warnOnDifference(opts.Warn, name, baseName, record, data); err != nil {
				return nil, fmt.Errorf("%s: %w", ref.URI, err)
			}
		}

		if _, err := coll.AddWebPostcard(name+".postcard."+ext, data); err != nil {
			return nil, fmt.Errorf("%s: %w", ref.URI, err)
		}
		names = append(names, name)
	}

	return names, nil
}

func uniqueName(name string, taken map[string]bool) string {
	candidate := name
	for n := 2; taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", name, n)
	}
	taken[candidate] = true
	return candidate
}

// baseName is the name before any clash suffix: it's what the record's title derives from.
func warnOnDifference(warn func(string), name, baseName string, record atproto.Record, data []byte) error {
	pc, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), name).Decode(formats.DecodeOptions{})
	if err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}
	if diffs := atproto.Diff(atproto.FromMetadata(pc.Meta, record.Image), record, baseName); len(diffs) > 0 {
		warn(fmt.Sprintf("%s: record and image metadata differ: %s", name, strings.Join(diffs, ", ")))
	}
	return nil
}
