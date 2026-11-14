package atproto

import (
	"fmt"
	"strings"
)

// CollectionRecordType is the value of a collection record's "$type" field.
const CollectionRecordType = "org.dotpostcard.collection"

// StrongRef is a com.atproto.repo.strongRef: a record's at-uri pinned to a version by its CID.
type StrongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// Collection is an org.dotpostcard.collection record: a titled, ordered list of references
// to org.dotpostcard.postcard records.
type Collection struct {
	Type      string      `json:"$type"`
	Title     string      `json:"title"`
	Postcards []StrongRef `json:"postcards"`
	CreatedAt string      `json:"createdAt"`
}

// ParseATURI splits an at://{authority}/{collection}/{rkey} URI into its parts, rejecting
// malformed URIs and invalid record keys.
func ParseATURI(uri string) (authority, collection, rkey string, err error) {
	rest, ok := strings.CutPrefix(uri, "at://")
	if !ok {
		return "", "", "", fmt.Errorf("%q is not an at:// URI", uri)
	}

	parts := strings.SplitN(rest, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("%q is not an at://{authority}/{collection}/{rkey} URI", uri)
	}

	if err := ValidRecordKey(parts[2]); err != nil {
		return "", "", "", fmt.Errorf("%q: %w", uri, err)
	}

	return parts[0], parts[1], parts[2], nil
}

// FetchCollection resolves the collection at uri, then fetches each postcard record it
// references, anonymously. The returned records are parallel to Collection.Postcards. A record
// whose current CID differs from its reference's is still returned, as records can be updated.
// pdsHostOverride and plcHostOverride are as for Anonymous.
func FetchCollection(uri, pdsHostOverride, plcHostOverride string) (Collection, []Record, error) {
	authority, nsid, rkey, err := ParseATURI(uri)
	if err != nil {
		return Collection{}, nil, err
	}
	if nsid != CollectionRecordType {
		return Collection{}, nil, fmt.Errorf("%q is not an %s record URI", uri, CollectionRecordType)
	}

	client, err := Anonymous(authority, pdsHostOverride, plcHostOverride)
	if err != nil {
		return Collection{}, nil, err
	}

	var collection Collection
	if _, err := client.GetRecordIn(client.DID(), CollectionRecordType, rkey, &collection); err != nil {
		return Collection{}, nil, fmt.Errorf("fetching collection %s: %w", uri, err)
	}

	clients := map[string]*Client{authority: client}
	records := make([]Record, len(collection.Postcards))
	for i, ref := range collection.Postcards {
		refAuthority, refNSID, refRkey, err := ParseATURI(ref.URI)
		if err != nil {
			return Collection{}, nil, err
		}
		if refNSID != RecordType {
			return Collection{}, nil, fmt.Errorf("%q is not an %s record URI", ref.URI, RecordType)
		}

		rc, ok := clients[refAuthority]
		if !ok {
			if rc, err = Anonymous(refAuthority, pdsHostOverride, plcHostOverride); err != nil {
				return Collection{}, nil, err
			}
			clients[refAuthority] = rc
		}

		if _, err := rc.GetRecordIn(rc.DID(), RecordType, refRkey, &records[i]); err != nil {
			return Collection{}, nil, fmt.Errorf("fetching postcard %s: %w", ref.URI, err)
		}
	}

	return collection, records, nil
}
