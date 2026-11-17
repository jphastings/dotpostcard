package appcore

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdscollection"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/internal/testpds"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPDS starts a fake PDS, points appcore at it, and returns a client logged in to it, plus a
// count of the blob downloads the PDS has served.
func newPDS(t *testing.T) (*atproto.Client, *atomic.Int32) {
	t.Helper()
	srv := testpds.New("did:plc:alice")
	t.Cleanup(srv.Close)

	blobGets := &atomic.Int32{}
	inner := srv.Config.Handler
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "com.atproto.sync.getBlob") {
			blobGets.Add(1)
		}
		inner.ServeHTTP(w, r)
	})

	atprotoPDSHost = srv.URL
	t.Cleanup(func() { atprotoPDSHost = "" })

	client, err := atproto.Login("alice.example", "pw", srv.URL, "")
	require.NoError(t, err)
	return client, blobGets
}

// publishCollection publishes a collection titled title, holding the named cards.
func publishCollection(t *testing.T, client *atproto.Client, title string, names ...string) (uri string, cards []collection.CardSummary) {
	t.Helper()
	coll, err := collection.Create(filepath.Join(t.TempDir(), "source.postcards"))
	require.NoError(t, err)
	t.Cleanup(func() { coll.Close() })
	require.NoError(t, coll.SetTitle(title))

	for _, name := range names {
		pc := testhelpers.SamplePostcard
		pc.Name = name
		pc.Meta.Title = name
		pc.Meta.SentOn = nil
		pc.Meta.Front.Secrets = nil
		pc.Meta.Back.Secrets = nil
		pc.Front = testhelpers.TestImages["sample-front.png"]
		pc.Back = testhelpers.TestImages["sample-back.png"]
		fws, err := web.DefaultCodec.Encode(pc, nil)
		require.NoError(t, err)
		data, err := fws[0].Bytes()
		require.NoError(t, err)
		_, err = coll.AddWebPostcard(fws[0].Filename, data)
		require.NoError(t, err)
	}

	result, err := pdscollection.Publish(client, coll, pdscollection.PublishOptions{})
	require.NoError(t, err)
	cards, err = coll.List()
	require.NoError(t, err)
	return result.URI, cards
}

func TestAtprotoCollectionsJSON(t *testing.T) {
	client, _ := newPDS(t)

	empty, err := AtprotoCollectionsJSON("alice.example")
	require.NoError(t, err)
	assert.Equal(t, "[]", empty)

	uri, _ := publishCollection(t, client, "Trip", "pyramids", "harbour")
	var stored atproto.Collection
	cid, err := client.GetRecordIn(client.DID(), atproto.CollectionRecordType, "trip", &stored)
	require.NoError(t, err)

	out, err := AtprotoCollectionsJSON("alice.example")
	require.NoError(t, err)
	assert.JSONEq(t, `[{"uri":"`+uri+`","cid":"`+cid+`","title":"Trip","cardCount":2,"createdAt":"`+stored.CreatedAt+`"}]`, out)
}

func TestAtprotoFollowsJSON(t *testing.T) {
	client, _ := newPDS(t)

	empty, err := AtprotoFollowsJSON("alice.example")
	require.NoError(t, err)
	assert.Equal(t, "[]", empty)

	subject := "at://did:plc:bob/org.dotpostcard.collection/trip"
	_, err = client.PutRecordIn(atproto.FollowRecordType, "3labc", atproto.Follow{
		Type: atproto.FollowRecordType, Subject: subject, CreatedAt: "2026-01-02T03:04:05Z",
	})
	require.NoError(t, err)

	out, err := AtprotoFollowsJSON("alice.example")
	require.NoError(t, err)
	assert.JSONEq(t, `[{"uri":"at://did:plc:alice/org.dotpostcard.follow/3labc","subject":"`+subject+`","createdAt":"2026-01-02T03:04:05Z"}]`, out)
}

func TestFetchAtprotoCollectionWritesTheCollection(t *testing.T) {
	client, _ := newPDS(t)
	uri, published := publishCollection(t, client, "Trip", "pyramids", "harbour")
	dest := filepath.Join(t.TempDir(), "trip.postcards")

	out, err := FetchAtprotoCollection(uri, dest)
	require.NoError(t, err)

	var fetched []collection.CardSummary
	require.NoError(t, json.Unmarshal([]byte(out), &fetched))
	hashes := map[string]string{}
	for _, card := range fetched {
		hashes[card.Name] = card.SHA256
	}
	wantHashes := map[string]string{}
	for _, card := range published {
		wantHashes[card.Name] = card.SHA256
	}
	assert.Equal(t, wantHashes, hashes)
	assert.Len(t, hashes, 2)

	col, err := OpenCollection(dest)
	require.NoError(t, err)
	defer col.Close()
	title, err := col.Title()
	require.NoError(t, err)
	assert.Equal(t, "Trip", title)
	listed, err := col.ListJSON()
	require.NoError(t, err)
	assert.JSONEq(t, out, listed)
}

func TestFetchAtprotoCollectionFailureLeavesDestAlone(t *testing.T) {
	client, _ := newPDS(t)
	_, _ = publishCollection(t, client, "Trip", "pyramids")

	var stored atproto.Collection
	_, err := client.GetRecordIn(client.DID(), atproto.CollectionRecordType, "trip", &stored)
	require.NoError(t, err)
	stored.Postcards = append(stored.Postcards, atproto.StrongRef{URI: "at://did:plc:alice/org.dotpostcard.postcard/missing", CID: "bafy"})
	_, err = client.PutRecordIn(atproto.CollectionRecordType, "broken", stored)
	require.NoError(t, err)

	dir := t.TempDir()
	dest := filepath.Join(dir, "trip.postcards")
	require.NoError(t, os.WriteFile(dest, []byte("previous"), 0o644))

	_, err = FetchAtprotoCollection("at://did:plc:alice/org.dotpostcard.collection/broken", dest)
	require.Error(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "previous", string(got))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".postcards-fetch-"), e.Name())
	}
}

func TestFetchAtprotoCollectionReusesImagesAlreadyInDestPath(t *testing.T) {
	client, blobGets := newPDS(t)
	uri, _ := publishCollection(t, client, "Trip", "pyramids", "harbour")
	dest := filepath.Join(t.TempDir(), "trip.postcards")
	_, err := FetchAtprotoCollection(uri, dest)
	require.NoError(t, err)
	assert.EqualValues(t, 2, blobGets.Load())

	publishCollection(t, client, "Trip", "pyramids", "harbour", "castle")
	blobGets.Store(0)
	out, err := FetchAtprotoCollection(uri, dest)
	require.NoError(t, err)

	assert.EqualValues(t, 1, blobGets.Load())
	var cards []collection.CardSummary
	require.NoError(t, json.Unmarshal([]byte(out), &cards))
	assert.Len(t, cards, 3)
}

func TestAtprotoCollectionJSON(t *testing.T) {
	client, _ := newPDS(t)
	publishCollection(t, client, "Trip", "pyramids")

	listed, err := AtprotoCollectionsJSON("alice.example")
	require.NoError(t, err)
	var all []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(listed), &all))
	require.Len(t, all, 1)

	one, err := AtprotoCollectionJSON("at://alice.example/org.dotpostcard.collection/trip")
	require.NoError(t, err)
	assert.JSONEq(t, string(all[0]), one)
}

func TestAtprotoCollectionJSONMissingRecord(t *testing.T) {
	newPDS(t)

	_, err := AtprotoCollectionJSON("at://alice.example/org.dotpostcard.collection/nope")
	assert.Error(t, err)
}
