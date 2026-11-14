// Package pdscollection publishes a *.postcards collection to an atproto PDS as an
// org.dotpostcard.collection record, and builds a collection back from one. It lives apart
// from formats/atproto because it needs SQLite and the web codec.
package pdscollection

import (
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

// PublishedCard reports one card's postcard record. Reused is true when a record already
// existed at the card's key (in a dry run: when it would be reused rather than created).
// In a dry run a card that doesn't exist yet has no CID.
type PublishedCard struct {
	Name   string
	URI    string
	CID    string
	Reused bool
}

// PublishResult reports what Publish did. URI is empty only in a dry run of a collection that
// has never been published. Unchanged means the collection record already matched, so it was
// not written.
type PublishResult struct {
	URI       string
	Cards     []PublishedCard
	Unchanged bool
}

// Publish uploads every card of coll as a postcard record, then writes (or updates) the
// collection record listing them. Records are keyed so that publishing the same collection
// again reuses them, and writes nothing if the collection record is unchanged.
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
	rkey, storedURI, err := collectionKey(client, coll)
	if err != nil {
		return PublishResult{}, err
	}

	var existing atproto.Collection
	_, err = client.GetRecordIn(client.DID(), atproto.CollectionRecordType, rkey, &existing)
	hasExisting := err == nil
	if err != nil && !errors.Is(err, atproto.ErrRecordNotFound) {
		return PublishResult{}, fmt.Errorf("reading the collection record: %w", err)
	}

	var result PublishResult
	refs := make([]atproto.StrongRef, 0, len(cards))
	for _, card := range cards {
		published, err := publishCard(client, coll, card, opts.DryRun)
		if err != nil {
			return PublishResult{}, fmt.Errorf("%s: %w", card.Name, err)
		}
		result.Cards = append(result.Cards, published)
		refs = append(refs, atproto.StrongRef{URI: published.URI, CID: published.CID})
		if opts.OnCard != nil {
			opts.OnCard(published)
		}
	}

	if title == "" {
		title = opts.DefaultTitle
	}
	result.URI = fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.CollectionRecordType, rkey)
	if storedURI == "" && opts.DryRun {
		result.URI = ""
	}

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
		return PublishResult{}, err
	}
	return result, nil
}

func publishCard(client *atproto.Client, coll *collection.Collection, card collection.CardSummary, dryRun bool) (PublishedCard, error) {
	data, mimetype, err := coll.CardData(card.Name)
	if err != nil {
		return PublishedCard{}, err
	}

	if !dryRun {
		uri, cid, reused, err := pdsupload.UploadWebFile(client, data, mimetype, card.Name, card.AddedAt)
		return PublishedCard{Name: card.Name, URI: uri, CID: cid, Reused: reused}, err
	}

	rkey, _, err := pdsupload.WebFileKey(data, card.Name, card.AddedAt)
	if err != nil {
		return PublishedCard{}, err
	}
	published := PublishedCard{Name: card.Name, URI: fmt.Sprintf("at://%s/%s/%s", client.DID(), atproto.RecordType, rkey)}
	_, published.CID, err = client.GetRecord(client.DID(), rkey)
	if err != nil && !errors.Is(err, atproto.ErrRecordNotFound) {
		return PublishedCard{}, err
	}
	published.Reused = err == nil
	return published, nil
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

// collectionKey is the collection record's key: that of its stored published URI, or a fresh
// TID if it has never been published.
func collectionKey(client *atproto.Client, coll *collection.Collection) (rkey, storedURI string, err error) {
	storedURI, err = coll.PublishedURI()
	if err != nil {
		return "", "", err
	}
	if storedURI == "" {
		return atproto.NewTID(time.Now()), "", nil
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
