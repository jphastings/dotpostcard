package atproto

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"
	"unicode"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/types"
)

var _ formats.Bundle = Bundle{}

// Bundle reads a single org.dotpostcard.postcard record from a PDS.
type Bundle struct {
	client *Client
	did    string
	rkey   string
	uri    string
	warn   func(string)
	record Record
	name   string
}

// ParseURI splits an at://{authority}/org.dotpostcard.postcard/{rkey} URI into its authority
// and record key, rejecting other collections or malformed URIs. Any syntactically valid rkey
// is accepted here (not just TIDs), so records keyed the old, pre-TID way still resolve.
func ParseURI(uri string) (authority, rkey string, err error) {
	rest, ok := strings.CutPrefix(uri, "at://")
	if !ok {
		return "", "", fmt.Errorf("%q is not an at:// URI", uri)
	}

	parts := strings.SplitN(rest, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] != RecordType {
		return "", "", fmt.Errorf("%q is not an %s record URI (want at://{authority}/%s/{rkey})", uri, RecordType, RecordType)
	}

	if err := ValidRecordKey(parts[2]); err != nil {
		return "", "", fmt.Errorf("%q: %w", uri, err)
	}

	return parts[0], parts[2], nil
}

// NewBundle resolves uri's authority (skipping resolution if pdsHostOverride is set; using
// plcHostOverride in place of the default PLC directory for a did:plc) and fetches its record
// eagerly, so CardName() is available before Decode. warn is called with a description of any
// mismatch found between the record and its image's own metadata.
func NewBundle(uri, pdsHostOverride, plcHostOverride string, warn func(string)) (Bundle, error) {
	authority, rkey, err := ParseURI(uri)
	if err != nil {
		return Bundle{}, err
	}

	client, err := Anonymous(authority, pdsHostOverride, plcHostOverride)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolving %q: %w", authority, err)
	}

	record, cid, err := client.GetRecord(client.DID(), rkey)
	if err != nil {
		return Bundle{}, fmt.Errorf("fetching record: %w", err)
	}

	return Bundle{
		client: client,
		did:    client.DID(),
		rkey:   rkey,
		uri:    uri,
		warn:   warn,
		record: record,
		name:   nameFromRecord(record, cid),
	}, nil
}

func (b Bundle) RefPath() string   { return b.uri }
func (b Bundle) CardName() string  { return b.name }
func (b Bundle) CodecName() string { return "ATProto" }

// nameFromRecord names a downloaded card from its record's location (kebab-cased), falling
// back to the start of the record's own CID when there's no usable location — the rkey is a
// TID and not fit for human use as a name.
const cidNameLength = 12

func nameFromRecord(record Record, cid string) string {
	if record.Location != nil {
		if name := kebabCase(record.Location.Name); name != "" {
			return name
		}
	}

	// Every record CID shares this prefix (CIDv1, dag-cbor, sha256), so it distinguishes nothing.
	unique := strings.TrimPrefix(cid, "bafyrei")
	if len(unique) > cidNameLength {
		return unique[:cidNameLength]
	}
	return unique
}

// kebabCase lower-cases s, keeping unicode letters and digits, and collapses every run of
// other characters to a single "-".
func kebabCase(s string) string {
	var b strings.Builder
	lastDash := true // suppresses a leading dash
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// Decode fetches the record's image blob, decodes it with the web codec, and returns metadata
// from the record (the record wins over anything the image's own XMP disagrees with, which is
// reported to warn) merged with the pixel-derived fields the record can't carry.
func (b Bundle) Decode(decOpts formats.DecodeOptions) (types.Postcard, error) {
	blobBytes, err := b.client.GetBlob(b.did, b.record.Image.Ref.Link)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("fetching image blob: %w", err)
	}

	pc, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(blobBytes)), b.name).Decode(decOpts)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("decoding image blob: %w", err)
	}

	recordMeta, err := b.record.ToMetadata()
	if err != nil {
		return types.Postcard{}, fmt.Errorf("interpreting record: %w", err)
	}

	if b.warn != nil {
		imageRecord := FromMetadata(pc.Meta, b.record.Image)
		if diffs := Diff(imageRecord, b.record); len(diffs) > 0 {
			b.warn(fmt.Sprintf("record and image metadata differ: %s", strings.Join(diffs, ", ")))
		}
	}

	// The record wins, but pixel-derived fields it can't carry come from the decoded image.
	recordMeta.Name = b.name
	recordMeta.HasTransparency = pc.Meta.HasTransparency
	recordMeta.Physical.FrontDimensions.PxWidth = pc.Meta.Physical.FrontDimensions.PxWidth
	recordMeta.Physical.FrontDimensions.PxHeight = pc.Meta.Physical.FrontDimensions.PxHeight

	if pc.Meta.Physical.FrontDimensions.HasPhysical() && recordMeta.Physical.FrontDimensions.HasPhysical() &&
		roundedMm(pc.Meta.Physical.FrontDimensions.CmWidth) == roundedMm(recordMeta.Physical.FrontDimensions.CmWidth) &&
		roundedMm(pc.Meta.Physical.FrontDimensions.CmHeight) == roundedMm(recordMeta.Physical.FrontDimensions.CmHeight) {
		recordMeta.Physical.FrontDimensions.CmWidth = pc.Meta.Physical.FrontDimensions.CmWidth
		recordMeta.Physical.FrontDimensions.CmHeight = pc.Meta.Physical.FrontDimensions.CmHeight
	}

	pc.Meta = recordMeta
	pc.Name = b.name

	return pc, nil
}

func roundedMm(cm *big.Rat) int {
	mm, _ := new(big.Rat).Mul(cm, big.NewRat(10, 1)).Float64()
	return int(math.Round(mm))
}
