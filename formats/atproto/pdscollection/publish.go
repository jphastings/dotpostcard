// Package pdscollection publishes a *.postcards collection to an atproto PDS as an
// org.dotpostcard.collection record, and builds a collection back from one. It lives apart
// from formats/atproto because it needs SQLite and the web codec.
package pdscollection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdsupload"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/jphastings/dotpostcard/types"
)

// maxTitleBytes is the collection lexicon's limit on title.
const maxTitleBytes = 3000

// PublishOptions configures Publish.
type PublishOptions struct {
	// DryRun reads from the PDS but writes nothing, not even the collection's stored URI.
	DryRun bool
	// DefaultTitle titles the collection record when the collection has no title of its own
	// (the CLI passes the file's stem).
	DefaultTitle string
	// OnCard, if set, is called as each card is handled, in collection order.
	OnCard func(PublishedCard)
}

// PublishedCard reports one card's postcard record. Reused is true when the account already
// had a record with the card's exact image, whatever its key (in a dry run: when it would be
// reused rather than created). In a dry run a card that doesn't exist yet has no CID.
type PublishedCard struct {
	Name   string
	URI    string
	CID    string
	Reused bool
}

// PublishResult reports what Publish did. URI is the collection record's address, known even
// in a dry run. Unchanged means the collection record already matched, so it was not written.
type PublishResult struct {
	URI       string
	Cards     []PublishedCard
	Unchanged bool
}

// Publish uploads every card of coll as a postcard record, then writes (or updates) the
// collection record listing them. A card whose image is already in the account's postcard
// records is reused, so publishing the same collection again uploads nothing, and writes
// nothing if the collection record is unchanged.
func Publish(client *atproto.Client, coll *collection.Collection, opts PublishOptions) (PublishResult, error) {
	cards, err := coll.List()
	if err != nil {
		return PublishResult{}, err
	}

	if err := refuseUnhiddenSecrets(coll, cards); err != nil {
		return PublishResult{}, err
	}

	title, err := coll.Title()
	if err != nil {
		return PublishResult{}, err
	}
	if title == "" {
		title = opts.DefaultTitle
	}
	if len(title) > maxTitleBytes {
		return PublishResult{}, fmt.Errorf("title is %d bytes; the limit is %d", len(title), maxTitleBytes)
	}
	rkey, storedURI, err := collectionKey(client, coll, title)
	if err != nil {
		return PublishResult{}, err
	}

	var existing atproto.Collection
	_, err = client.GetRecordIn(client.DID(), atproto.CollectionRecordType, rkey, &existing)
	hasExisting := err == nil
	if err != nil && !errors.Is(err, atproto.ErrRecordNotFound) {
		return PublishResult{}, fmt.Errorf("reading the collection record: %w", err)
	}

	listed, err := listedByImage(client, existing.Postcards)
	if err != nil {
		return PublishResult{}, err
	}

	var result PublishResult
	refs := make([]atproto.StrongRef, 0, len(cards))
	for _, card := range cards {
		published, err := publishCard(client, coll, card, listed, opts.DryRun)
		if err != nil {
			return PublishResult{}, fmt.Errorf("%s: %w", card.Name, err)
		}
		result.Cards = append(result.Cards, published)
		refs = append(refs, atproto.StrongRef{URI: published.URI, CID: published.CID})
		if opts.OnCard != nil {
			opts.OnCard(published)
		}
	}

	result.URI = fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.CollectionRecordType, rkey)

	if hasExisting && existing.Title == title && slices.Equal(existing.Postcards, refs) {
		result.Unchanged = true
		return result, nil
	}
	if opts.DryRun {
		return result, nil
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	if hasExisting {
		createdAt = existing.CreatedAt
	}
	record := atproto.Collection{Type: atproto.CollectionRecordType, Title: title, Postcards: refs, CreatedAt: createdAt}
	// Stored before the write so a failed or unacknowledged put is retried at the same key,
	// not duplicated under a new one.
	if storedURI == "" {
		if err := coll.SetPublishedURI(result.URI); err != nil {
			return PublishResult{}, err
		}
	}
	if _, err := client.PutRecordIn(atproto.CollectionRecordType, rkey, record); err != nil {
		// Nobody knows yet how a PDS refuses an oversized record, so any failure names the size.
		encoded, _ := json.Marshal(record)
		return PublishResult{}, fmt.Errorf("writing the collection record (cards: %d, %.1f KB; a collection is always one record, it isn't split): %w", len(refs), float64(len(encoded))/1024, err)
	}
	return result, nil
}

// listedByImage maps each of the account's postcard records by its image's sha256. Where
// several share an image it prefers one the collection already points at, else the lowest URI.
func listedByImage(client *atproto.Client, inCollection []atproto.StrongRef) (map[[sha256.Size]byte]atproto.StrongRef, error) {
	listed := map[[sha256.Size]byte]atproto.StrongRef{}
	for rec, err := range client.ListRecords(client.DID(), atproto.RecordType) {
		if err != nil {
			return nil, fmt.Errorf("listing existing postcard records: %w", err)
		}
		var record atproto.Record
		if json.Unmarshal(rec.Value, &record) != nil {
			continue
		}
		sum, err := record.Image.SHA256()
		if err != nil {
			continue
		}
		ref := atproto.StrongRef{URI: rec.URI, CID: rec.CID}
		if prev, ok := listed[sum]; ok && !betterRef(ref, prev, inCollection) {
			continue
		}
		listed[sum] = ref
	}
	return listed, nil
}

func betterRef(candidate, current atproto.StrongRef, inCollection []atproto.StrongRef) bool {
	isIn := func(ref atproto.StrongRef) bool {
		return slices.ContainsFunc(inCollection, func(r atproto.StrongRef) bool { return r.URI == ref.URI })
	}
	if isIn(candidate) != isIn(current) {
		return isIn(candidate)
	}
	return candidate.URI < current.URI
}

func publishCard(client *atproto.Client, coll *collection.Collection, card collection.CardSummary, listed map[[sha256.Size]byte]atproto.StrongRef, dryRun bool) (PublishedCard, error) {
	data, mimetype, err := coll.CardData(card.Name)
	if err != nil {
		return PublishedCard{}, err
	}

	if ref, ok := listed[sha256.Sum256(data)]; ok {
		return PublishedCard{Name: card.Name, URI: ref.URI, CID: ref.CID, Reused: true}, nil
	}

	if !dryRun {
		uri, cid, reused, err := pdsupload.UploadWebFile(client, data, mimetype, card.Name, card.AddedAt)
		return PublishedCard{Name: card.Name, URI: uri, CID: cid, Reused: reused}, err
	}

	rkey, _, err := pdsupload.WebFileKey(data, card.Name, card.AddedAt)
	if err != nil {
		return PublishedCard{}, err
	}
	return PublishedCard{Name: card.Name, URI: fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.RecordType, rkey)}, nil
}

// refuseUnhiddenSecrets stops a publish that would expose a secret: cards are uploaded exactly
// as stored, so a secret region that isn't already obscured in the image would be public.
func refuseUnhiddenSecrets(coll *collection.Collection, cards []collection.CardSummary) error {
	var exposed []string
	for _, card := range cards {
		meta, err := coll.Metadata(card.Name)
		if err != nil {
			return err
		}
		for _, secrets := range [][]types.Polygon{meta.Front.Secrets, meta.Back.Secrets} {
			if slices.ContainsFunc(secrets, func(p types.Polygon) bool { return !p.Prehidden }) {
				exposed = append(exposed, card.Name)
				break
			}
		}
	}
	if len(exposed) > 0 {
		return fmt.Errorf("not publishing: these cards have secret regions that aren't pre-hidden in the image, so publishing would reveal them: %s", strings.Join(exposed, ", "))
	}
	return nil
}

// titleKey names a collection after its title, for both its file and its record, so publishing
// the same title again updates the same record. Titles with no Latin letters or digits have no
// slug, so they fall back to a hash of the title.
func titleKey(title string) string {
	if slug := types.Slug(title); slug != "" {
		return slug
	}
	sum := sha256.Sum256([]byte(title))
	return hex.EncodeToString(sum[:6])
}

// collectionKey is the collection record's key: that of its stored published URI, or else the
// key of its title.
func collectionKey(client *atproto.Client, coll *collection.Collection, title string) (rkey, storedURI string, err error) {
	storedURI, err = coll.PublishedURI()
	if err != nil {
		return "", "", err
	}
	if storedURI == "" {
		if title == "" {
			return "", "", errors.New("the collection has no title to key its record by: set one with `collection title`")
		}
		rkey = titleKey(title)
		return rkey, "", atproto.ValidRecordKey(rkey)
	}

	authority, nsid, rkey, err := atproto.ParseATURI(storedURI)
	if err != nil {
		return "", "", fmt.Errorf("the collection's stored published URI: %w", err)
	}
	if nsid != atproto.CollectionRecordType {
		return "", "", fmt.Errorf("the collection's stored published URI %q is not an %s record", storedURI, atproto.CollectionRecordType)
	}
	if authority != client.DID() {
		return "", "", fmt.Errorf("this collection was published as %s, but you are logged in as %s; log in as that account to update it", authority, client.DID())
	}
	return rkey, storedURI, nil
}
