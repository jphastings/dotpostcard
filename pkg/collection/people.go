package collection

import (
	"database/sql"
	"fmt"
	"sort"
)

// PersonRef is a distinct person referenced somewhere in a collection, along
// with every role they're referenced in.
type PersonRef struct {
	Name  string   `json:"name,omitempty"`
	Uri   string   `json:"uri,omitempty"`
	Roles []string `json:"roles"` // subset of "from", "to", "collector"
}

// personKey identifies a PersonRef for deduplication: the same name with a
// different uri (or vice versa) is a different person, mirroring how
// SearchFiltered treats name and URI matches as distinct concepts.
type personKey struct {
	name string
	uri  string
}

// roleOrder fixes the order roles are appended in when merging, so
// PersonRef.Roles is deterministic regardless of the order rows are scanned.
var roleOrder = []string{"from", "to", "collector"}

// People returns every distinct person referenced across the collection's
// cards, as a sender, recipient, or context/collector author, together with
// every role they appear in.
//
// It's read with json_extract(cards.metadata_json, ...) rather than a
// Go-side decode of every card's metadata: both SQLite drivers this codebase
// builds against (modernc.org/sqlite for desktop/test builds,
// github.com/mattn/go-sqlite3 for the gomobile build) bundle SQLite >= 3.38,
// where the JSON functions (including json_extract) are compiled into the
// core build unconditionally (SQLITE_OMIT_JSON is unset in both), so
// json_extract is always available.
func (c *Collection) People() ([]PersonRef, error) {
	rows, err := c.db.Query(`
		SELECT json_extract(metadata_json, '$.sender.name') AS name, json_extract(metadata_json, '$.sender.uri') AS uri, 'from' AS role FROM cards
		UNION ALL
		SELECT json_extract(metadata_json, '$.recipient.name'), json_extract(metadata_json, '$.recipient.uri'), 'to' FROM cards
		UNION ALL
		SELECT json_extract(metadata_json, '$.context.author.name'), json_extract(metadata_json, '$.context.author.uri'), 'collector' FROM cards
	`)
	if err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	defer rows.Close()

	dedup := make(map[personKey]*PersonRef)
	for rows.Next() {
		var name, uri sql.NullString
		var role string
		if err := rows.Scan(&name, &uri, &role); err != nil {
			return nil, fmt.Errorf("listing people: %w", err)
		}

		if name.String == "" && uri.String == "" {
			continue
		}

		key := personKey{name: name.String, uri: uri.String}
		ref, ok := dedup[key]
		if !ok {
			ref = &PersonRef{Name: name.String, Uri: uri.String}
			dedup[key] = ref
		}
		addRole(ref, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}

	// personKey uniquely determines (Name, Uri), so sorting by those fields
	// alone is enough for deterministic output; no tie-break on insertion
	// order is needed.
	people := make([]PersonRef, 0, len(dedup))
	for _, ref := range dedup {
		people = append(people, *ref)
	}

	sort.Slice(people, func(i, j int) bool {
		if people[i].Name != people[j].Name {
			return people[i].Name < people[j].Name
		}
		return people[i].Uri < people[j].Uri
	})

	return people, nil
}

// addRole appends role to ref.Roles if it isn't already present, keeping
// roles in the fixed roleOrder rather than scan order.
func addRole(ref *PersonRef, role string) {
	for _, existing := range ref.Roles {
		if existing == role {
			return
		}
	}
	ref.Roles = append(ref.Roles, role)
	sort.Slice(ref.Roles, func(i, j int) bool {
		return roleIndex(ref.Roles[i]) < roleIndex(ref.Roles[j])
	})
}

func roleIndex(role string) int {
	for i, r := range roleOrder {
		if r == role {
			return i
		}
	}
	return len(roleOrder)
}
