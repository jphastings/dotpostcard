package appcore

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdscollection"
	"github.com/jphastings/dotpostcard/pkg/collection"
)

// atprotoPDSHost and atprotoPLCHost override identity resolution, as for atproto.Anonymous; only tests set them.
var atprotoPDSHost, atprotoPLCHost string

type atprotoCollection struct {
	URI       string   `json:"uri"`
	CID       string   `json:"cid"`
	Title     string   `json:"title"`
	CardCount int      `json:"cardCount"`
	CreatedAt string   `json:"createdAt"`
	Postcards []string `json:"postcards"`
}

func newAtprotoCollection(uri, cid string, record atproto.Collection) atprotoCollection {
	postcards := make([]string, len(record.Postcards))
	for i, ref := range record.Postcards {
		postcards[i] = ref.URI
	}
	return atprotoCollection{
		URI:       uri,
		CID:       cid,
		Title:     record.Title,
		CardCount: len(record.Postcards),
		CreatedAt: record.CreatedAt,
		Postcards: postcards,
	}
}

type atprotoFollow struct {
	URI       string `json:"uri"`
	Subject   string `json:"subject"`
	CreatedAt string `json:"createdAt"`
}

// AtprotoCollectionsJSON lists the org.dotpostcard.collection records in actor's repo (actor is a
// handle or DID), anonymously, as a JSON array of {"uri","cid","title","cardCount","createdAt","postcards"},
// where postcards lists the at-uris of the collection's postcard records, in order.
func AtprotoCollectionsJSON(actor string) (string, error) {
	client, err := atproto.Anonymous(actor, atprotoPDSHost, atprotoPLCHost)
	if err != nil {
		return "", err
	}

	var collections []atprotoCollection
	for listed, err := range client.ListRecords(client.DID(), atproto.CollectionRecordType) {
		if err != nil {
			return "", fmt.Errorf("listing collections of %s: %w", actor, err)
		}
		var record atproto.Collection
		if err := json.Unmarshal(listed.Value, &record); err != nil {
			return "", fmt.Errorf("reading collection %s: %w", listed.URI, err)
		}
		collections = append(collections, newAtprotoCollection(listed.URI, listed.CID, record))
	}
	return marshalJSONArray(collections)
}

// AtprotoFollowsJSON lists the org.dotpostcard.follow records in actor's repo, anonymously, as a
// JSON array of {"uri","subject","createdAt"}. uri is the follow record's own, which deleting unfollows.
func AtprotoFollowsJSON(actor string) (string, error) {
	client, err := atproto.Anonymous(actor, atprotoPDSHost, atprotoPLCHost)
	if err != nil {
		return "", err
	}

	var follows []atprotoFollow
	for listed, err := range client.ListRecords(client.DID(), atproto.FollowRecordType) {
		if err != nil {
			return "", fmt.Errorf("listing follows of %s: %w", actor, err)
		}
		var record atproto.Follow
		if err := json.Unmarshal(listed.Value, &record); err != nil {
			return "", fmt.Errorf("reading follow %s: %w", listed.URI, err)
		}
		follows = append(follows, atprotoFollow{URI: listed.URI, Subject: record.Subject, CreatedAt: record.CreatedAt})
	}
	return marshalJSONArray(follows)
}

// FetchProgress is told about each card as FetchAtprotoCollection commits it to the file.
type FetchProgress interface {
	// CardAdded is called once per postcard record the collection references, in the collection's
	// order, after its card (named name) is committed. done counts the references handled so far,
	// of total.
	CardAdded(uri, name string, done, total int)
}

// FetchAtprotoCollection downloads the org.dotpostcard.collection at uri into a .postcards file at
// destPath, anonymously, built as `postcards collection create at://…` builds one. The file is
// built in place and each card is committed as it arrives, so a reader (OpenCollection) always
// sees a valid, growing collection; progress, if not nil, is told about each card as it lands.
// A failure leaves the cards committed so far, and calling it again carries on, reusing images
// already in the file. Cards the record no longer lists are removed only after every card has
// arrived.
//
// It returns a JSON array of {"uri","name"}, one per postcard record the collection references,
// in the collection's order: the record's at-uri, and the name of its card in the written file
// (open it with the Collection methods). Names can change between fetches; URIs don't.
func FetchAtprotoCollection(uri, destPath string, progress FetchProgress) (string, error) {
	cache := &imageCache{path: destPath}
	defer cache.close()

	var cards []fetchedCard
	err := pdscollection.Sync(destPath, uri, pdscollection.BuildOptions{
		PDSHost: atprotoPDSHost,
		PLCHost: atprotoPLCHost,
		Cached:  cache.lookup,
		OnRecord: func(uri, name string, done, total int) {
			cards = append(cards, fetchedCard{URI: uri, Name: name})
			if progress != nil {
				progress.CardAdded(uri, name, done, total)
			}
		},
	})
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", uri, err)
	}
	return marshalJSONArray(cards)
}

type fetchedCard struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// imageCache serves the images of the collection file at path by sha256. The file is opened and
// indexed on the first lookup; a missing or unreadable file is just an empty cache. It's safe for
// concurrent use.
type imageCache struct {
	mu     sync.Mutex
	path   string
	loaded bool
	col    *collection.Collection
	names  map[[sha256.Size]byte]string
}

func (c *imageCache) load() {
	c.loaded = true
	c.names = map[[sha256.Size]byte]string{}
	col, err := collection.OpenReadOnly(c.path)
	if err != nil {
		return
	}
	names, err := col.CardNamesBySHA256()
	if err != nil {
		col.Close()
		return
	}
	c.col, c.names = col, names
}

func (c *imageCache) lookup(sum [sha256.Size]byte) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.load()
	}
	name, ok := c.names[sum]
	if !ok {
		return nil, false
	}
	data, _, err := c.col.CardData(name)
	return data, err == nil
}

// close releases the file; lookups that are still in flight after it are misses.
func (c *imageCache) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.col != nil {
		c.col.Close()
	}
	c.col, c.names, c.loaded = nil, nil, true
}

// AtprotoCollectionJSON reads the one org.dotpostcard.collection record at uri, anonymously, as the
// same JSON object AtprotoCollectionsJSON lists.
func AtprotoCollectionJSON(uri string) (string, error) {
	authority, nsid, rkey, err := atproto.ParseATURI(uri)
	if err != nil {
		return "", err
	}
	if nsid != atproto.CollectionRecordType {
		return "", fmt.Errorf("%q is not an %s record URI", uri, atproto.CollectionRecordType)
	}

	client, err := atproto.Anonymous(authority, atprotoPDSHost, atprotoPLCHost)
	if err != nil {
		return "", err
	}

	var record atproto.Collection
	cid, err := client.GetRecordIn(client.DID(), atproto.CollectionRecordType, rkey, &record)
	if err != nil {
		return "", fmt.Errorf("reading collection %s: %w", uri, err)
	}
	return marshalJSON(newAtprotoCollection(fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.CollectionRecordType, rkey), cid, record))
}
