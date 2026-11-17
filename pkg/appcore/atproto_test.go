package appcore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdscollection"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/internal/testpds"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pdsProbe watches, and can disturb, the record and blob requests a fake PDS serves.
type pdsProbe struct {
	blobGets atomic.Int32
	// peak is the most getRecord/getBlob requests that were in flight at once.
	peak     atomic.Int32
	inFlight atomic.Int32
	// hold makes each getRecord/getBlob request last at least this long; set it before fetching.
	hold      time.Duration
	failBlobs sync.Map
}

// failBlob makes getBlob for cid return an error.
func (p *pdsProbe) failBlob(cid string) { p.failBlobs.Store(cid, true) }

func (p *pdsProbe) allowBlobs() { p.failBlobs.Clear() }

// newPDS starts a fake PDS, points appcore at it, and returns a client logged in to it, plus a
// probe of the requests it serves.
func newPDS(t *testing.T) (*atproto.Client, *pdsProbe) {
	t.Helper()
	srv := testpds.New("did:plc:alice")
	t.Cleanup(srv.Close)

	probe := &pdsProbe{}
	inner := srv.Config.Handler
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isBlob := strings.HasSuffix(r.URL.Path, "com.atproto.sync.getBlob")
		if isBlob || strings.HasSuffix(r.URL.Path, "com.atproto.repo.getRecord") {
			n := probe.inFlight.Add(1)
			defer probe.inFlight.Add(-1)
			for peak := probe.peak.Load(); n > peak && !probe.peak.CompareAndSwap(peak, n); peak = probe.peak.Load() {
			}
			time.Sleep(probe.hold)
		}
		if isBlob {
			probe.blobGets.Add(1)
			if _, fail := probe.failBlobs.Load(r.URL.Query().Get("cid")); fail {
				http.Error(w, `{"error":"InternalServerError"}`, http.StatusInternalServerError)
				return
			}
		}
		inner.ServeHTTP(w, r)
	})

	atprotoPDSHost = srv.URL
	t.Cleanup(func() { atprotoPDSHost = "" })

	client, err := atproto.Login("alice.example", "pw", srv.URL, "")
	require.NoError(t, err)
	return client, probe
}

// publishCollection publishes a collection titled title, holding the named cards.
func publishCollection(t *testing.T, client *atproto.Client, title string, names ...string) string {
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
	return result.URI
}

// storedCollection reads back the collection record at rkey in alice's repo.
func storedCollection(t *testing.T, client *atproto.Client, rkey string) (atproto.Collection, string) {
	t.Helper()
	var record atproto.Collection
	cid, err := client.GetRecordIn(client.DID(), atproto.CollectionRecordType, rkey, &record)
	require.NoError(t, err)
	return record, cid
}

func postcardURIs(record atproto.Collection) []string {
	uris := make([]string, len(record.Postcards))
	for i, ref := range record.Postcards {
		uris[i] = ref.URI
	}
	return uris
}

// requireFetched checks that every postcard URI maps to a card that opens in the file at dest.
func requireFetched(t *testing.T, out, dest string, wantURIs []string) []fetchedCard {
	t.Helper()
	var cards []fetchedCard
	require.NoError(t, json.Unmarshal([]byte(out), &cards))
	uris := make([]string, len(cards))
	for i, card := range cards {
		uris[i] = card.URI
	}
	require.ElementsMatch(t, wantURIs, uris)

	col, err := OpenCollection(dest)
	require.NoError(t, err)
	defer col.Close()
	for _, card := range cards {
		_, err := col.CardImage(card.Name)
		require.NoError(t, err, card.Name)
	}
	return cards
}

func TestAtprotoCollectionsJSON(t *testing.T) {
	client, _ := newPDS(t)

	empty, err := AtprotoCollectionsJSON("alice.example")
	require.NoError(t, err)
	assert.Equal(t, "[]", empty)

	uri := publishCollection(t, client, "Trip", "pyramids", "harbour")
	stored, cid := storedCollection(t, client, "trip")
	postcards, err := json.Marshal(postcardURIs(stored))
	require.NoError(t, err)

	out, err := AtprotoCollectionsJSON("alice.example")
	require.NoError(t, err)
	assert.JSONEq(t, `[{"uri":"`+uri+`","cid":"`+cid+`","title":"Trip","cardCount":2,"createdAt":"`+stored.CreatedAt+`","postcards":`+string(postcards)+`}]`, out)
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
	uri := publishCollection(t, client, "Trip", "pyramids", "harbour")
	stored, _ := storedCollection(t, client, "trip")
	dest := filepath.Join(t.TempDir(), "trip.postcards")

	out, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)

	cards := requireFetched(t, out, dest, postcardURIs(stored))
	assert.Len(t, cards, 2)
	col, err := OpenCollection(dest)
	require.NoError(t, err)
	defer col.Close()
	title, err := col.Title()
	require.NoError(t, err)
	assert.Equal(t, "Trip", title)
}

func TestFetchAtprotoCollectionMapsIdenticalCardsToOneName(t *testing.T) {
	client, _ := newPDS(t)
	publishCollection(t, client, "Trip", "pyramids")
	stored, _ := storedCollection(t, client, "trip")
	_, _, rkey, err := atproto.ParseATURI(stored.Postcards[0].URI)
	require.NoError(t, err)
	record, _, err := client.GetRecord(client.DID(), rkey)
	require.NoError(t, err)
	dupCID, err := client.PutRecord("duplicate", record)
	require.NoError(t, err)

	stored.Postcards = []atproto.StrongRef{
		stored.Postcards[0],
		{URI: "at://" + client.DID() + "/" + atproto.RecordType + "/duplicate", CID: dupCID},
	}
	_, err = client.PutRecordIn(atproto.CollectionRecordType, "twins", stored)
	require.NoError(t, err)
	dest := filepath.Join(t.TempDir(), "twins.postcards")

	out, err := FetchAtprotoCollection("at://alice.example/org.dotpostcard.collection/twins", dest, nil)
	require.NoError(t, err)

	cards := requireFetched(t, out, dest, postcardURIs(stored))
	assert.Equal(t, cards[0].Name, cards[1].Name)
	col, err := OpenCollection(dest)
	require.NoError(t, err)
	defer col.Close()
	count, err := col.CardCount()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestFetchAtprotoCollectionReusesImagesAlreadyInDestPath(t *testing.T) {
	client, pds := newPDS(t)
	uri := publishCollection(t, client, "Trip", "pyramids", "harbour")
	dest := filepath.Join(t.TempDir(), "trip.postcards")
	_, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 2, pds.blobGets.Load())

	publishCollection(t, client, "Trip", "pyramids", "harbour", "castle")
	stored, _ := storedCollection(t, client, "trip")
	pds.blobGets.Store(0)
	out, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)

	assert.EqualValues(t, 1, pds.blobGets.Load())
	assert.Len(t, requireFetched(t, out, dest, postcardURIs(stored)), 3)
}

func TestAtprotoCollectionJSON(t *testing.T) {
	client, _ := newPDS(t)
	publishCollection(t, client, "Trip", "pyramids")
	stored, _ := storedCollection(t, client, "trip")

	listed, err := AtprotoCollectionsJSON("alice.example")
	require.NoError(t, err)
	var all []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(listed), &all))
	require.Len(t, all, 1)

	one, err := AtprotoCollectionJSON("at://alice.example/org.dotpostcard.collection/trip")
	require.NoError(t, err)
	assert.JSONEq(t, string(all[0]), one)
	assert.Contains(t, one, `"postcards":["`+stored.Postcards[0].URI+`"]`)
}

func TestAtprotoCollectionJSONMissingRecord(t *testing.T) {
	newPDS(t)

	_, err := AtprotoCollectionJSON("at://alice.example/org.dotpostcard.collection/nope")
	assert.Error(t, err)
}

type progressFunc func(uri, name string, done, total int)

func (f progressFunc) CardAdded(uri, name string, done, total int) { f(uri, name, done, total) }

// blobCID is the CID of the image blob of the postcard record ref points at.
func blobCID(t *testing.T, client *atproto.Client, ref atproto.StrongRef) string {
	t.Helper()
	_, _, rkey, err := atproto.ParseATURI(ref.URI)
	require.NoError(t, err)
	record, _, err := client.GetRecord(client.DID(), rkey)
	require.NoError(t, err)
	return record.Image.Ref.Link
}

func cardNames(t *testing.T, path string) []string {
	t.Helper()
	col, err := collection.OpenReadOnly(path)
	require.NoError(t, err)
	defer col.Close()
	cards, err := col.List()
	require.NoError(t, err)
	names := make([]string, len(cards))
	for i, card := range cards {
		names[i] = card.Name
	}
	return names
}

func TestFetchAtprotoCollectionReportsProgressPerReference(t *testing.T) {
	client, _ := newPDS(t)
	uri := publishCollection(t, client, "Trip", "pyramids", "harbour", "castle")
	stored, _ := storedCollection(t, client, "trip")
	dest := filepath.Join(t.TempDir(), "trip.postcards")

	type step struct{ done, total int }
	var uris []string
	var steps []step
	out, err := FetchAtprotoCollection(uri, dest, progressFunc(func(uri, name string, done, total int) {
		uris = append(uris, uri)
		steps = append(steps, step{done, total})
	}))
	require.NoError(t, err)

	assert.Equal(t, postcardURIs(stored), uris)
	assert.Equal(t, []step{{1, 3}, {2, 3}, {3, 3}}, steps)
	assert.Len(t, requireFetched(t, out, dest, uris), 3)
}

func TestFetchAtprotoCollectionIsReadableWhileFetching(t *testing.T) {
	client, _ := newPDS(t)
	uri := publishCollection(t, client, "Trip", "pyramids", "harbour", "castle")
	dest := filepath.Join(t.TempDir(), "trip.postcards")

	var reader *Collection
	t.Cleanup(func() {
		if reader != nil {
			reader.Close()
		}
	})
	_, err := FetchAtprotoCollection(uri, dest, progressFunc(func(_, _ string, done, _ int) {
		if reader == nil {
			var err error
			reader, err = OpenCollection(dest)
			require.NoError(t, err)
		}
		count, err := reader.CardCount()
		require.NoError(t, err)
		assert.EqualValues(t, done, count)
		_, err = reader.ListJSON()
		require.NoError(t, err)
	}))

	require.NoError(t, err)
}

func TestFetchAtprotoCollectionResumesAfterAnInterruption(t *testing.T) {
	client, pds := newPDS(t)
	uri := publishCollection(t, client, "Trip", "pyramids", "harbour", "castle", "lighthouse")
	stored, _ := storedCollection(t, client, "trip")
	dest := filepath.Join(t.TempDir(), "trip.postcards")
	pds.failBlob(blobCID(t, client, stored.Postcards[2]))

	_, err := FetchAtprotoCollection(uri, dest, nil)
	require.Error(t, err)
	assert.Len(t, cardNames(t, dest), 2)

	pds.allowBlobs()
	pds.blobGets.Store(0)
	out, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)

	assert.EqualValues(t, 2, pds.blobGets.Load())
	assert.Len(t, requireFetched(t, out, dest, postcardURIs(stored)), 4)
	assert.Len(t, cardNames(t, dest), 4)
}

func TestFetchAtprotoCollectionRefreshReplacesDroppedCards(t *testing.T) {
	client, pds := newPDS(t)
	uri := publishCollection(t, client, "Trip", "pyramids", "harbour", "castle")
	dest := filepath.Join(t.TempDir(), "trip.postcards")
	_, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)

	publishCollection(t, client, "Trip", "pyramids", "castle", "lighthouse")
	stored, _ := storedCollection(t, client, "trip")
	pds.blobGets.Store(0)
	out, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)

	assert.EqualValues(t, 1, pds.blobGets.Load())
	assert.ElementsMatch(t, []string{"pyramids", "castle", "lighthouse"}, cardNames(t, dest))
	requireFetched(t, out, dest, postcardURIs(stored))
}

func TestFetchAtprotoCollectionFailedRefreshKeepsEveryCard(t *testing.T) {
	client, pds := newPDS(t)
	uri := publishCollection(t, client, "Trip", "pyramids", "harbour", "castle")
	dest := filepath.Join(t.TempDir(), "trip.postcards")
	_, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)

	publishCollection(t, client, "Trip", "pyramids", "castle", "lighthouse")
	stored, _ := storedCollection(t, client, "trip")
	for _, ref := range stored.Postcards {
		pds.failBlob(blobCID(t, client, ref))
	}
	_, err = FetchAtprotoCollection(uri, dest, nil)
	require.Error(t, err)

	assert.ElementsMatch(t, []string{"pyramids", "harbour", "castle"}, cardNames(t, dest))
}

func TestFetchAtprotoCollectionFetchesAtMostSixCardsAtOnce(t *testing.T) {
	client, pds := newPDS(t)
	names := make([]string, 8)
	for i := range names {
		names[i] = fmt.Sprintf("card%02d", i)
	}
	uri := publishCollection(t, client, "Trip", names...)
	dest := filepath.Join(t.TempDir(), "trip.postcards")
	pds.hold = 10 * time.Millisecond

	_, err := FetchAtprotoCollection(uri, dest, nil)
	require.NoError(t, err)

	assert.Len(t, cardNames(t, dest), 8)
	assert.LessOrEqual(t, pds.peak.Load(), int32(6))
	assert.Greater(t, pds.peak.Load(), int32(1))
}
