package atproto

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jphastings/dotpostcard/formats/atproto/internal/testpds"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const collectionFixture = `{
  "$type": "org.dotpostcard.collection",
  "title": "Blank postcards",
  "postcards": [
    {"uri": "at://did:plc:example/org.dotpostcard.postcard/3jzfcijpj2z2a", "cid": "bafyrei1"},
    {"uri": "at://did:plc:example/org.dotpostcard.postcard/3jzfcijpj2z2b", "cid": "bafyrei2"}
  ],
  "createdAt": "2026-11-14T00:00:00Z"
}`

func TestCollectionRoundTripsThroughGoTypes(t *testing.T) {
	var c Collection
	require.NoError(t, json.Unmarshal([]byte(collectionFixture), &c))

	out, err := json.Marshal(c)
	require.NoError(t, err)
	assert.JSONEq(t, collectionFixture, string(out))
}

func TestParseATURI(t *testing.T) {
	authority, collection, rkey, err := ParseATURI("at://did:plc:abc/app.bsky.feed.post/3jzfcijpj2z2a")
	require.NoError(t, err)
	assert.Equal(t, []string{"did:plc:abc", "app.bsky.feed.post", "3jzfcijpj2z2a"}, []string{authority, collection, rkey})

	for _, bad := range []string{"https://example.com", "at://did:plc:abc", "at://did:plc:abc/coll"} {
		_, _, _, err := ParseATURI(bad)
		assert.Error(t, err, bad)
	}
}

func TestFetchCollectionReturnsPostcardsInListOrder(t *testing.T) {
	srv := testpds.New("did:plc:testuser")
	defer srv.Close()

	client, err := Login("did:plc:testuser", "pw", srv.URL, "")
	require.NoError(t, err)

	var refs []StrongRef
	for _, rkey := range []string{"zzz", "aaa"} {
		record := FromMetadata(types.Metadata{Locale: "en-GB"}, testBlob)
		record.Sides = []Side{{Description: rkey}}
		cid, err := client.PutRecord(rkey, record)
		require.NoError(t, err)
		refs = append(refs, StrongRef{URI: fmt.Sprintf("at://did:plc:testuser/%s/%s", RecordType, rkey), CID: cid})
	}
	// An out-of-date CID must not stop the record being returned.
	refs[1].CID = "bafyreistale"

	_, err = client.PutRecordIn(CollectionRecordType, "3jzfcijpj2z2a", Collection{Type: CollectionRecordType, Title: "Mine", Postcards: refs, CreatedAt: "2026-11-14T00:00:00Z"})
	require.NoError(t, err)

	collection, records, err := FetchCollection("at://did:plc:testuser/"+CollectionRecordType+"/3jzfcijpj2z2a", srv.URL, "")
	require.NoError(t, err)

	assert.Equal(t, "Mine", collection.Title)
	require.Len(t, records, 2)
	assert.Equal(t, "zzz", records[0].Sides[0].Description)
	assert.Equal(t, "aaa", records[1].Sides[0].Description)
}

func TestListRecordsPagesThroughEverything(t *testing.T) {
	srv := testpds.New("did:plc:testuser")
	defer srv.Close()

	client, err := Login("did:plc:testuser", "pw", srv.URL, "")
	require.NoError(t, err)

	for i := range 250 {
		_, err := client.PutRecordIn("test.coll", fmt.Sprintf("r%03d", i), map[string]int{"n": i})
		require.NoError(t, err)
	}

	var rkeys []string
	for rec, err := range client.ListRecords("did:plc:testuser", "test.coll") {
		require.NoError(t, err)
		rkeys = append(rkeys, rec.URI)
	}

	require.Len(t, rkeys, 250)
	assert.Equal(t, "at://did:plc:testuser/test.coll/r000", rkeys[0])
	assert.Equal(t, "at://did:plc:testuser/test.coll/r249", rkeys[249])
}
