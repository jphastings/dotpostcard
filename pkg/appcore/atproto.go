package appcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdscollection"
	"github.com/jphastings/dotpostcard/pkg/collection"
)

// atprotoPDSHost and atprotoPLCHost override identity resolution, as for atproto.Anonymous; only tests set them.
var atprotoPDSHost, atprotoPLCHost string

type atprotoCollection struct {
	URI       string `json:"uri"`
	CID       string `json:"cid"`
	Title     string `json:"title"`
	CardCount int    `json:"cardCount"`
	CreatedAt string `json:"createdAt"`
}

type atprotoFollow struct {
	URI       string `json:"uri"`
	Subject   string `json:"subject"`
	CreatedAt string `json:"createdAt"`
}

// AtprotoCollectionsJSON lists the org.dotpostcard.collection records in actor's repo (actor is a
// handle or DID), anonymously, as a JSON array of {"uri","cid","title","cardCount","createdAt"}.
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
		collections = append(collections, atprotoCollection{
			URI:       listed.URI,
			CID:       listed.CID,
			Title:     record.Title,
			CardCount: len(record.Postcards),
			CreatedAt: record.CreatedAt,
		})
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

// FetchAtprotoCollection downloads the org.dotpostcard.collection at uri into a .postcards file at
// destPath, anonymously, built exactly as `postcards collection create at://…` builds one. It's
// built in a temporary directory beside destPath and renamed into place, so a failed fetch leaves
// any existing file at destPath untouched. It returns the written file's cards as a JSON array, the
// same shape as Collection.ListJSON; each card's "sha256" (its image's hash, unchanged from the
// published blob) identifies it stably across fetches. Images already in the file at destPath
// are reused rather than downloaded again.
func FetchAtprotoCollection(uri, destPath string) (string, error) {
	stage, err := os.MkdirTemp(filepath.Dir(destPath), ".postcards-fetch-*")
	if err != nil {
		return "", fmt.Errorf("preparing to fetch %s: %w", uri, err)
	}
	defer os.RemoveAll(stage)

	cache := openImageCache(destPath)
	result, err := pdscollection.Create([]string{uri}, pdscollection.CreateOptions{
		BuildOptions: pdscollection.BuildOptions{PDSHost: atprotoPDSHost, PLCHost: atprotoPLCHost, Cached: cache.lookup},
		WriteFile:    true,
		OutDir:       stage,
	})
	cache.close()
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", uri, err)
	}

	if err := os.Rename(result.Path, destPath); err != nil {
		return "", fmt.Errorf("moving fetched collection into place: %w", err)
	}

	col, err := collection.OpenReadOnly(destPath)
	if err != nil {
		return "", fmt.Errorf("opening fetched collection: %w", err)
	}
	defer col.Close()

	cards, err := col.List()
	if err != nil {
		return "", fmt.Errorf("listing fetched cards: %w", err)
	}
	return marshalJSONArray(cards)
}

// imageCache serves the images of an existing collection file by sha256. A missing or unreadable
// file is just an empty cache.
type imageCache struct {
	col   *collection.Collection
	names map[string]string
}

func openImageCache(path string) *imageCache {
	cache := &imageCache{names: map[string]string{}}
	col, err := collection.OpenReadOnly(path)
	if err != nil {
		return cache
	}
	cards, err := col.List()
	if err != nil {
		col.Close()
		return cache
	}
	cache.col = col
	for _, card := range cards {
		cache.names[card.SHA256] = card.Name
	}
	return cache
}

func (c *imageCache) lookup(sum [sha256.Size]byte) ([]byte, bool) {
	name, ok := c.names[hex.EncodeToString(sum[:])]
	if !ok {
		return nil, false
	}
	data, _, err := c.col.CardData(name)
	return data, err == nil
}

func (c *imageCache) close() {
	if c.col != nil {
		c.col.Close()
	}
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
	return marshalJSON(atprotoCollection{
		URI:       fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.CollectionRecordType, rkey),
		CID:       cid,
		Title:     record.Title,
		CardCount: len(record.Postcards),
		CreatedAt: record.CreatedAt,
	})
}
