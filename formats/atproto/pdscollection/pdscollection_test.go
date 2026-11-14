package pdscollection

import (
	"crypto/sha256"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/internal/testpds"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/sqldb"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testServer struct {
	url    string
	writes *atomic.Int32
	close  func()
}

// newServer starts a fake PDS that counts the writes (blob uploads and record puts) it gets.
func newServer(t *testing.T) testServer {
	t.Helper()
	srv := testpds.New("did:plc:testuser")
	writes := &atomic.Int32{}
	inner := srv.Config.Handler
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "uploadBlob") || strings.HasSuffix(r.URL.Path, "putRecord") {
			writes.Add(1)
		}
		inner.ServeHTTP(w, r)
	})
	t.Cleanup(srv.Close)
	return testServer{url: srv.URL, writes: writes}
}

func login(t *testing.T, srv testServer) *atproto.Client {
	t.Helper()
	client, err := atproto.Login("alice.example", "app-password", srv.url, "")
	require.NoError(t, err)
	return client
}

func addCard(t *testing.T, coll *collection.Collection, name, title string) {
	t.Helper()
	pc := testhelpers.SamplePostcard
	pc.Name = name
	pc.Meta.Title = title
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

func newCollection(t *testing.T) (*collection.Collection, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blanks.postcards")
	coll, err := collection.Create(path)
	require.NoError(t, err)
	t.Cleanup(func() { coll.Close() })
	require.NoError(t, coll.SetTitle("Blank postcards"))
	addCard(t, coll, "pyramids", "Pyramids")
	addCard(t, coll, "harbour", "Harbour")
	return coll, path
}

func TestPublishIsIdempotent(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := newCollection(t)

	first, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)
	assert.Len(t, first.Cards, 2)
	assert.EqualValues(t, 5, srv.writes.Load(), "two blobs, two postcards, one collection")
	stored, err := coll.PublishedURI()
	require.NoError(t, err)
	assert.Equal(t, first.URI, stored)

	srv.writes.Store(0)
	second, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)
	assert.EqualValues(t, 0, srv.writes.Load())
	assert.True(t, second.Unchanged)
	assert.Equal(t, first.URI, second.URI)
	for _, card := range second.Cards {
		assert.True(t, card.Reused)
	}
}

func TestRemovedCardLeavesItsRecord(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := newCollection(t)
	result, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)

	require.NoError(t, coll.Remove("harbour"))
	_, err = Publish(client, coll, PublishOptions{})
	require.NoError(t, err)

	var published atproto.Collection
	_, err = client.GetRecordIn(client.DID(), atproto.CollectionRecordType, result.URI[strings.LastIndex(result.URI, "/")+1:], &published)
	require.NoError(t, err)
	assert.Len(t, published.Postcards, 1)

	records := 0
	for _, err := range client.ListRecords(client.DID(), atproto.RecordType) {
		require.NoError(t, err)
		records++
	}
	assert.Equal(t, 2, records)
}

func TestUnhiddenSecretStopsPublishBeforeAnyWrite(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, path := newCollection(t)

	pc := testhelpers.SamplePostcard
	pc.Name = "secretive"
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	fws, err := web.DefaultCodec.Encode(pc, nil)
	require.NoError(t, err)
	data, err := fws[0].Bytes()
	require.NoError(t, err)
	_, err = coll.AddWebPostcard(fws[0].Filename, data)
	require.NoError(t, err)

	db, err := sqldb.Open(path, false)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE cards SET metadata_json = REPLACE(metadata_json, '"prehidden":true', '"prehidden":false') WHERE name = 'secretive'`)
	require.NoError(t, err)
	db.Close()

	_, err = Publish(client, coll, PublishOptions{})
	assert.ErrorContains(t, err, "secretive")
	assert.EqualValues(t, 0, srv.writes.Load())
}

func TestDryRunWritesNothing(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := newCollection(t)

	result, err := Publish(client, coll, PublishOptions{DryRun: true})
	require.NoError(t, err)
	assert.Len(t, result.Cards, 2)
	assert.EqualValues(t, 0, srv.writes.Load())
	stored, err := coll.PublishedURI()
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func TestPublishRefusesAnotherAccountsCollection(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := newCollection(t)
	require.NoError(t, coll.SetPublishedURI("at://did:plc:someoneelse/org.dotpostcard.collection/3lxyzabcdef2c"))

	_, err := Publish(client, coll, PublishOptions{})
	assert.ErrorContains(t, err, "did:plc:someoneelse")
	assert.EqualValues(t, 0, srv.writes.Load())
}

func TestFetchRebuildsThePublishedCollection(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := newCollection(t)
	result, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "fetched.postcards")
	var warnings []string
	_, err = Fetch(result.URI, out, FetchOptions{PDSHost: srv.url, Warn: func(m string) { warnings = append(warnings, m) }})
	require.NoError(t, err)
	assert.Empty(t, warnings)

	fetched, err := collection.OpenReadOnly(out)
	require.NoError(t, err)
	defer fetched.Close()

	title, err := fetched.Title()
	require.NoError(t, err)
	assert.Equal(t, "Blank postcards", title)

	want, err := coll.List()
	require.NoError(t, err)
	got, err := fetched.List()
	require.NoError(t, err)
	require.Len(t, got, len(want))
	for i := range want {
		assert.Equal(t, want[i].Name, got[i].Name)
		assert.Equal(t, want[i].Title, got[i].Title)

		wantData, _, err := coll.CardData(want[i].Name)
		require.NoError(t, err)
		gotData, _, err := fetched.CardData(got[i].Name)
		require.NoError(t, err)
		assert.Equal(t, sha256.Sum256(wantData), sha256.Sum256(gotData))
	}

	_, err = Fetch(result.URI, out, FetchOptions{PDSHost: srv.url})
	assert.ErrorContains(t, err, "already exists")
}

func TestFetchDoesNotWarnWhenANameClashSuffixesAnUntitledCard(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, err := collection.Create(filepath.Join(t.TempDir(), "clash.postcards"))
	require.NoError(t, err)
	defer coll.Close()
	addCard(t, coll, "aaa", "Harbour")
	addCard(t, coll, "harbour", "")
	result, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)

	var warnings []string
	names, err := Fetch(result.URI, filepath.Join(t.TempDir(), "fetched.postcards"), FetchOptions{PDSHost: srv.url, Warn: func(m string) { warnings = append(warnings, m) }})
	require.NoError(t, err)
	assert.Contains(t, names, "harbour-2")
	assert.Empty(t, warnings)
}
