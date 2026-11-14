package pdscollection

import (
	"crypto/sha256"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/internal/testpds"
	"github.com/jphastings/dotpostcard/formats/atproto/pdsupload"
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

func cardFile(t *testing.T, name, title string) (filename string, data []byte) {
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
	data, err = fws[0].Bytes()
	require.NoError(t, err)
	return fws[0].Filename, data
}

func addCard(t *testing.T, coll *collection.Collection, name, title string) {
	t.Helper()
	filename, data := cardFile(t, name, title)
	_, err := coll.AddWebPostcard(filename, data)
	require.NoError(t, err)
}

func writeCard(t *testing.T, dir, name, title string) string {
	t.Helper()
	filename, data := cardFile(t, name, title)
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func newCollection(t *testing.T) (*collection.Collection, string) {
	t.Helper()
	return collectionOf(t, "Blank postcards", "pyramids", "harbour")
}

func collectionOf(t *testing.T, title string, names ...string) (*collection.Collection, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.postcards")
	coll, err := collection.Create(path)
	require.NoError(t, err)
	t.Cleanup(func() { coll.Close() })
	require.NoError(t, coll.SetTitle(title))
	for _, name := range names {
		addCard(t, coll, name, strings.ToUpper(name[:1])+name[1:])
	}
	return coll, path
}

func cardBytes(t *testing.T, path string) map[string][]byte {
	t.Helper()
	coll, err := collection.OpenReadOnly(path)
	require.NoError(t, err)
	defer coll.Close()
	cards, err := coll.List()
	require.NoError(t, err)
	bytes := map[string][]byte{}
	for _, card := range cards {
		bytes[card.Name], _, err = coll.CardData(card.Name)
		require.NoError(t, err)
	}
	return bytes
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

func TestPublishedKeyFollowsTheTitleUntilAnAddressIsStored(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := newCollection(t)

	first, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(first.URI, "/blank-postcards"))

	require.NoError(t, coll.SetTitle("Renamed"))
	second, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)
	assert.Equal(t, first.URI, second.URI)
}

func TestDryRunKnowsTheAddressOfAnUnpublishedCollection(t *testing.T) {
	client := login(t, newServer(t))
	coll, _ := newCollection(t)

	result, err := Publish(client, coll, PublishOptions{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, "at://"+client.DID()+"/org.dotpostcard.collection/blank-postcards", result.URI)
}

func TestTitleWithoutLatinLettersStillKeysARecord(t *testing.T) {
	client := login(t, newServer(t))
	coll, _ := collectionOf(t, "絵葉書", "pyramids")

	result, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)
	_, _, rkey, err := atproto.ParseATURI(result.URI)
	require.NoError(t, err)
	assert.NoError(t, atproto.ValidRecordKey(rkey))
	assert.NotEmpty(t, rkey)
}

func TestCreateGathersEveryKindOfSource(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	published, _ := collectionOf(t, "Remote", "epsilon", "zeta")
	remote, err := Publish(client, published, PublishOptions{})
	require.NoError(t, err)

	dir := t.TempDir()
	alone := writeCard(t, dir, "alpha", "Alpha")
	inDir := filepath.Join(dir, "scans")
	require.NoError(t, os.Mkdir(inDir, 0o755))
	writeCard(t, inDir, "beta", "Beta")
	require.NoError(t, os.WriteFile(filepath.Join(inDir, "notes.txt"), []byte("ignored"), 0o644))
	_, packed := collectionOf(t, "Packed", "gamma")

	out := t.TempDir()
	result, err := Create(
		[]string{alone, inDir, packed, remote.URI, remote.Cards[1].URI},
		CreateOptions{BuildOptions: BuildOptions{PDSHost: srv.url}, Title: "Mixed", WriteFile: true, OutDir: out},
	)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(out, "mixed.postcards"), result.Path)
	assert.Equal(t, []string{"alpha", "beta", "gamma", "epsilon", "zeta"}, result.Cards, "zeta, named twice with identical bytes, is stored once")
	assert.Len(t, cardBytes(t, result.Path), 5)
}

func TestCreateKeepsBytesUnlessAQualityIsGiven(t *testing.T) {
	dir := t.TempDir()
	path := writeCard(t, dir, "alpha", "Alpha")
	original, err := os.ReadFile(path)
	require.NoError(t, err)

	untouched, err := Create([]string{path}, CreateOptions{Title: "Same", WriteFile: true, OutDir: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, original, cardBytes(t, untouched.Path)["alpha"])

	detail := formats.Detail
	reencoded, err := Create([]string{path}, CreateOptions{BuildOptions: BuildOptions{Quality: &detail}, Title: "Same", WriteFile: true, OutDir: t.TempDir()})
	require.NoError(t, err)
	assert.NotEqual(t, original, cardBytes(t, reencoded.Path)["alpha"])
}

func TestCreatingTwiceWithOneTitleUpdatesOneRecord(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	card := writeCard(t, t.TempDir(), "alpha", "Alpha")
	opts := CreateOptions{Title: "Trip", Client: client}

	first, err := Create([]string{card}, opts)
	require.NoError(t, err)
	second, err := Create([]string{card}, opts)
	require.NoError(t, err)

	assert.Equal(t, first.Published.URI, second.Published.URI)
	assert.True(t, second.Published.Unchanged)
	assert.Empty(t, first.Path, "-f atproto alone leaves no file")
}

func TestCreateWritesTheFileBeforePublishingSoItStoresTheAddress(t *testing.T) {
	client := login(t, newServer(t))
	card := writeCard(t, t.TempDir(), "alpha", "Alpha")

	result, err := Create([]string{card}, CreateOptions{Title: "Trip", WriteFile: true, OutDir: t.TempDir(), Client: client})
	require.NoError(t, err)

	coll, err := collection.OpenReadOnly(result.Path)
	require.NoError(t, err)
	defer coll.Close()
	stored, err := coll.PublishedURI()
	require.NoError(t, err)
	assert.Equal(t, result.Published.URI, stored)
}

func TestCreateInheritsTheTitleOfASoleCollectionAndOtherwiseNeedsOne(t *testing.T) {
	_, packed := collectionOf(t, "Packed", "gamma")
	card := writeCard(t, t.TempDir(), "alpha", "Alpha")
	out := t.TempDir()

	result, err := Create([]string{packed}, CreateOptions{WriteFile: true, OutDir: out})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(out, "packed.postcards"), result.Path)

	_, err = Create([]string{card}, CreateOptions{WriteFile: true, OutDir: out})
	assert.ErrorContains(t, err, "title")
	_, err = Create([]string{packed, card}, CreateOptions{WriteFile: true, OutDir: out})
	assert.ErrorContains(t, err, "title")
}

func TestCreateSuffixesAClashingNameAndWarnsButStoresIdenticalCardsOnce(t *testing.T) {
	first, second, copyOfFirst := t.TempDir(), t.TempDir(), t.TempDir()
	writeCard(t, first, "same", "One")
	writeCard(t, second, "same", "Two")
	writeCard(t, copyOfFirst, "same", "One")

	var warnings []string
	result, err := Create([]string{first, second, copyOfFirst}, CreateOptions{
		BuildOptions: BuildOptions{Warn: func(m string) { warnings = append(warnings, m) }},
		Title:        "Clash", WriteFile: true, OutDir: t.TempDir(),
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"same", "same-2"}, result.Cards)
	assert.Len(t, warnings, 1)
}

func TestOverwritingDoesNotReimportThePreviousResult(t *testing.T) {
	dir := t.TempDir()
	writeCard(t, dir, "alpha", "Alpha")
	opts := CreateOptions{Title: "Trip", WriteFile: true, OutDir: dir}

	_, err := Create([]string{dir}, opts)
	require.NoError(t, err)
	_, err = Create([]string{dir}, opts)
	assert.ErrorContains(t, err, "already exists")

	opts.Overwrite = true
	result, err := Create([]string{dir}, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha"}, result.Cards)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "the card and the collection, no leftover build directory")
}

func TestCreateRefusesAnExplicitFileItCannotUse(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(notes, []byte("hi"), 0o644))

	_, err := Create([]string{notes}, CreateOptions{Title: "Trip", WriteFile: true, OutDir: t.TempDir()})
	assert.ErrorContains(t, err, "notes.txt")
}

func TestAddReplacesCardsAlreadyInTheCollection(t *testing.T) {
	_, path := collectionOf(t, "Trip", "alpha")
	dir := t.TempDir()
	writeCard(t, dir, "alpha", "Changed")
	writeCard(t, dir, "beta", "Beta")

	names, err := Add(path, []string{dir}, BuildOptions{})
	require.NoError(t, err)

	assert.Equal(t, []string{"alpha", "beta"}, names)
	assert.Len(t, cardBytes(t, path), 2)
}

func TestCreateRebuildsAPublishedCollectionFromItsAddress(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := newCollection(t)
	published, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)

	var warnings []string
	result, err := Create([]string{published.URI}, CreateOptions{
		BuildOptions: BuildOptions{PDSHost: srv.url, Warn: func(m string) { warnings = append(warnings, m) }},
		WriteFile:    true, OutDir: t.TempDir(),
	})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, "blank-postcards.postcards", filepath.Base(result.Path))

	fetched, err := collection.OpenReadOnly(result.Path)
	require.NoError(t, err)
	defer fetched.Close()
	title, err := fetched.Title()
	require.NoError(t, err)
	assert.Equal(t, "Blank postcards", title)

	for name, data := range cardBytes(t, result.Path) {
		want, _, err := coll.CardData(name)
		require.NoError(t, err)
		assert.Equal(t, sha256.Sum256(want), sha256.Sum256(data), name)
	}
}

func TestRebuildingDoesNotReportMetadataDifferencesWhenAClashSuffixesAnUntitledCard(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	coll, _ := collectionOf(t, "Clash")
	addCard(t, coll, "aaa", "Harbour")
	addCard(t, coll, "harbour", "")
	published, err := Publish(client, coll, PublishOptions{})
	require.NoError(t, err)

	var warnings []string
	result, err := Create([]string{published.URI}, CreateOptions{
		BuildOptions: BuildOptions{PDSHost: srv.url, Warn: func(m string) { warnings = append(warnings, m) }},
		WriteFile:    true, OutDir: t.TempDir(),
	})
	require.NoError(t, err)
	assert.Contains(t, result.Cards, "harbour-2")
	for _, w := range warnings {
		assert.NotContains(t, w, "differ")
	}
}

func TestPublishReusesARecordWithTheSameImageUnderAnotherKey(t *testing.T) {
	srv := newServer(t)
	client := login(t, srv)
	card := writeCard(t, t.TempDir(), "alpha", "Alpha")
	data, err := os.ReadFile(card)
	require.NoError(t, err)
	earlierURI, _, _, err := pdsupload.UploadWebFile(client, data, http.DetectContentType(data), "alpha", time.Now().AddDate(0, 0, -3))
	require.NoError(t, err)
	srv.writes.Store(0)

	dry, err := Create([]string{card}, CreateOptions{Title: "Trip", Client: client, Publish: PublishOptions{DryRun: true}})
	require.NoError(t, err)
	require.Len(t, dry.Published.Cards, 1)
	assert.Equal(t, PublishedCard{Name: "alpha", URI: earlierURI, CID: dry.Published.Cards[0].CID, Reused: true}, dry.Published.Cards[0])
	assert.EqualValues(t, 0, srv.writes.Load())

	real, err := Create([]string{card}, CreateOptions{Title: "Trip", Client: client})
	require.NoError(t, err)
	assert.Equal(t, earlierURI, real.Published.Cards[0].URI)
	assert.True(t, real.Published.Cards[0].Reused)
	assert.EqualValues(t, 1, srv.writes.Load(), "only the collection record")
}
