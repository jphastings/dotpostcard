package atproto

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/jphastings/dotpostcard/types"
)

// tidAlphabet is atproto's base32-sortable encoding.
const tidAlphabet = "234567abcdefghijklmnopqrstuvwxyz"

// tidEpoch is the earliest day a TID can represent: its 53-bit timestamp field can't hold a
// negative microsecond count.
var tidEpoch = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)

// RecordKey derives a TID (atproto's standard record key: a 64-bit value of a zero top bit, a
// 53-bit microsecond timestamp and a 10-bit clock id, base32-encoded to 13 characters) for a
// postcard.
//
// Its timestamp is the day the postcard was sent, or today's if that's unknown — clamped
// forward to 1970-01-01 for postcards sent before the Unix epoch, which is common for
// postcards. Its clock-id, and the time of day within that timestamp, come from the image's
// own hash, so re-uploading an unchanged image reproduces the same key.
func RecordKey(sentOn *types.Date, today time.Time, image []byte) string {
	// The same digest atproto embeds in the blob's CID: this genuinely is the image's hash,
	// not a separate identifier for it.
	return recordKeyFromDigest(sentOn, today, sha256.Sum256(image))
}

// recordKeyFromDigest is RecordKey's core: everything downstream of the image's sha256 digest,
// however that digest was obtained (hashing the bytes, or reading it out of a blob's CID).
func recordKeyFromDigest(sentOn *types.Date, today time.Time, sum [sha256.Size]byte) string {
	day := dayUTC(today)
	if sentOn != nil && !sentOn.IsZero() {
		day = dayUTC(sentOn.Time)
	}
	if day.Before(tidEpoch) {
		day = tidEpoch
	}

	h := binary.BigEndian.Uint64(sum[:8])

	micros := day.UnixMicro() + int64((h>>10)%86_400_000_000)
	clockID := h & 0x3ff

	return encodeTID(uint64(micros)<<10 | clockID)
}

// Key returns r's TID: derived from the day it was sent (r.SentOn, else today) plus the
// image's hash. It delegates to RecordKey.
func (r Record) Key(today time.Time, image []byte) string {
	return RecordKey(r.sentOnDate(), today, image)
}

// KeyFromImage returns r's TID (as Key would), reading the image's sha256 digest out of
// r.Image's own CID instead of hashing image bytes. This lets a caller that only holds the
// blob reference (eg. one that uploaded the blob in an earlier request) derive the same key
// it would have gotten from the bytes themselves.
//
// It errors if r.Image's CID is missing, or isn't the CIDv1/raw/sha2-256 shape atproto's
// uploadBlob always produces.
func (r Record) KeyFromImage(today time.Time) (string, error) {
	sum, err := blobDigest(r.Image)
	if err != nil {
		return "", err
	}
	return recordKeyFromDigest(r.sentOnDate(), today, sum), nil
}

func (r Record) sentOnDate() *types.Date {
	if r.SentOn == nil {
		return nil
	}
	return &types.Date{Time: time.Date(r.SentOn.Year, time.Month(r.SentOn.Month), r.SentOn.Day, 0, 0, 0, 0, time.UTC)}
}

// cidBase32 is multibase's "b" prefix's alphabet: RFC 4648 base32, lowercase, unpadded.
var cidBase32 = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

const (
	cidV1           = 1
	cidCodecRaw     = 0x55
	multihashSHA256 = 0x12
)

// blobDigest extracts the sha256 digest embedded in an atproto blob's CID. That CID is always
// CIDv1, raw codec, sha2-256 multihash, base32-lower multibase-encoded: <version><codec>
// <hash-fn><digest-length><digest>, prefixed with 'b'.
func blobDigest(b Blob) ([sha256.Size]byte, error) {
	link := b.Ref.Link
	if len(link) == 0 || link[0] != 'b' {
		return [sha256.Size]byte{}, fmt.Errorf("blob has no supported CID: %q", link)
	}

	raw, err := cidBase32.DecodeString(link[1:])
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("decoding CID %q: %w", link, err)
	}

	version, n := binary.Uvarint(raw)
	if n <= 0 {
		return [sha256.Size]byte{}, fmt.Errorf("decoding CID %q: bad version", link)
	}
	raw = raw[n:]

	codec, n := binary.Uvarint(raw)
	if n <= 0 {
		return [sha256.Size]byte{}, fmt.Errorf("decoding CID %q: bad codec", link)
	}
	raw = raw[n:]

	if version != cidV1 || codec != cidCodecRaw {
		return [sha256.Size]byte{}, fmt.Errorf("unsupported CID %q: want CIDv1 raw, got version %d codec 0x%x", link, version, codec)
	}

	hashFn, n := binary.Uvarint(raw)
	if n <= 0 {
		return [sha256.Size]byte{}, fmt.Errorf("decoding CID %q: bad multihash function", link)
	}
	raw = raw[n:]

	length, n := binary.Uvarint(raw)
	if n <= 0 {
		return [sha256.Size]byte{}, fmt.Errorf("decoding CID %q: bad multihash length", link)
	}
	raw = raw[n:]

	if hashFn != multihashSHA256 || length != sha256.Size || len(raw) != sha256.Size {
		return [sha256.Size]byte{}, fmt.Errorf("unsupported multihash in CID %q: want sha2-256/%d, got function 0x%x length %d", link, sha256.Size, hashFn, length)
	}

	var sum [sha256.Size]byte
	copy(sum[:], raw)
	return sum, nil
}

func dayUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func encodeTID(v uint64) string {
	var out [13]byte
	for i := 12; i >= 0; i-- {
		out[i] = tidAlphabet[v&0x1f]
		v >>= 5
	}
	return string(out[:])
}
