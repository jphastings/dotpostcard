package atproto

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encodeSamplePostcard produces the bytes of a real web-format postcard image, the way the
// CLI would before uploading it, along with the metadata that actually ended up embedded in
// it (Encode fills in pixel dimensions, and may resize, so what's embedded can differ from
// what went in — decoding it back is the only way to see what a downloader will see, which is
// also what the CLI must build the record from to avoid every download warning about it).
func encodeSamplePostcard(t *testing.T) ([]byte, string, types.Metadata) {
	t.Helper()

	pc := testhelpers.SamplePostcard
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]

	fws, err := web.DefaultCodec.Encode(pc, nil)
	require.NoError(t, err)
	require.Len(t, fws, 1)

	data, err := fws[0].Bytes()
	require.NoError(t, err)

	decoded, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), pc.Name).Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	return data, fws[0].Mimetype, decoded.Meta
}

// TestWebEncodingIsDeterministic checks the assumption the docs make (re-uploading an
// unchanged card hits the same TID, since RecordKey hashes the encoded image bytes): that
// encoding the same postcard twice with the web codec produces byte-identical output.
func TestWebEncodingIsDeterministic(t *testing.T) {
	dataA, _, _ := encodeSamplePostcard(t)
	dataB, _, _ := encodeSamplePostcard(t)

	assert.Equal(t, dataA, dataB, "web encoding must be deterministic for RecordKey's re-upload-hits-the-same-TID claim to hold")
}

func TestUploadDownloadRoundTrip(t *testing.T) {
	srv := newFakePDS("did:plc:testuser")
	defer srv.Close()

	client, err := Login("alice.example", "app-password", srv.URL, "")
	require.NoError(t, err)

	data, mimetype, meta := encodeSamplePostcard(t)

	blob, err := client.UploadBlob(data, mimetype)
	require.NoError(t, err)
	require.NoError(t, client.PutRecord("some-postcard", FromMetadata(meta, blob)))

	var warnings []string
	uri := fmt.Sprintf("at://%s/%s/some-postcard", client.DID(), RecordType)
	bundle, err := NewBundle(uri, srv.URL, "", func(msg string) { warnings = append(warnings, msg) })
	require.NoError(t, err)

	pc, err := bundle.Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	assert.Empty(t, warnings)
	// The card's name is derived from the record (its location, here), not the rkey.
	assert.Equal(t, "front-italy", pc.Name)
	assert.Equal(t, "front-italy", pc.Meta.Name)
	assert.Equal(t, meta.Sender, pc.Meta.Sender)
	assert.Equal(t, meta.Context, pc.Meta.Context)

	downloaded, err := client.GetBlob(client.DID(), blob.Ref.Link)
	require.NoError(t, err)
	assert.Equal(t, data, downloaded, "downloaded blob bytes must match what was uploaded")
}

func TestDecodeWarnsOnMismatchAndRecordWins(t *testing.T) {
	srv := newFakePDS("did:plc:testuser")
	defer srv.Close()

	client, err := Login("alice.example", "app-password", srv.URL, "")
	require.NoError(t, err)

	data, mimetype, meta := encodeSamplePostcard(t)
	blob, err := client.UploadBlob(data, mimetype)
	require.NoError(t, err)

	record := FromMetadata(meta, blob)
	record.Context.Description = "An edited description the image's own XMP doesn't know about"
	require.NoError(t, client.PutRecord("some-postcard", record))

	var warnings []string
	uri := fmt.Sprintf("at://%s/%s/some-postcard", client.DID(), RecordType)
	bundle, err := NewBundle(uri, srv.URL, "", func(msg string) { warnings = append(warnings, msg) })
	require.NoError(t, err)

	pc, err := bundle.Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "context")
	assert.Equal(t, record.Context.Description, pc.Meta.Context.Description, "the record's value must win over the image's")
}

func putAndFetchNamed(t *testing.T, locationName string) string {
	t.Helper()

	srv := newFakePDS("did:plc:testuser")
	defer srv.Close()

	client, err := Login("alice.example", "app-password", srv.URL, "")
	require.NoError(t, err)

	data, mimetype, meta := encodeSamplePostcard(t)
	meta.Location.Name = locationName

	blob, err := client.UploadBlob(data, mimetype)
	require.NoError(t, err)
	require.NoError(t, client.PutRecord("3jzfcijpj2z2a", FromMetadata(meta, blob)))

	uri := fmt.Sprintf("at://%s/%s/3jzfcijpj2z2a", client.DID(), RecordType)
	bundle, err := NewBundle(uri, srv.URL, "", nil)
	require.NoError(t, err)

	return bundle.CardName()
}

func TestNewBundleNamesCardFromLocation(t *testing.T) {
	assert.Equal(t, "san-marino", putAndFetchNamed(t, "San Marino"))
}

func TestNewBundleNamesCardFromMessyLocation(t *testing.T) {
	assert.Equal(t, "st-john-s-newfoundland", putAndFetchNamed(t, "  St. John's, Newfoundland "))
}

func TestNewBundleNamesCardFromCIDWhenNoLocation(t *testing.T) {
	assert.Equal(t, "fakerecordci", putAndFetchNamed(t, ""), "should be the start of the record CID, after its constant prefix")
}

func TestParseURIRejectsOtherCollectionsAndMalformedURIs(t *testing.T) {
	_, _, err := ParseURI("not-a-uri")
	assert.Error(t, err)

	_, _, err = ParseURI("at://did:plc:testuser/app.bsky.feed.post/abc123")
	assert.Error(t, err)

	authority, rkey, err := ParseURI("at://did:plc:testuser/org.dotpostcard.postcard/san-marino_1974")
	require.NoError(t, err)
	assert.Equal(t, "did:plc:testuser", authority)
	assert.Equal(t, "san-marino_1974", rkey)
}
