package appcore

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jphastings/dotpostcard/pkg/collection"
)

// Library is a brute-force, fan-out search index over a set of collections
// and bare card files. It does no central indexing of its own: every
// SearchJSON call re-queries each open collection's FTS index and re-scans
// the cached metadata of every bare file.
type Library struct {
	mu          sync.RWMutex
	collections map[string]*collection.Collection
	files       map[string]*CardFile
}

// NewLibrary creates an empty Library. Call SetSourcesJSON to populate it.
func NewLibrary() *Library {
	return &Library{
		collections: make(map[string]*collection.Collection),
		files:       make(map[string]*CardFile),
	}
}

type librarySources struct {
	Collections []string `json:"collections"`
	Cards       []string `json:"cards"`
}

// SetSourcesJSON replaces the Library's source set from a
// {"collections": [...], "cards": [...]} JSON object of absolute paths.
// Sources already open are reused rather than reopened; sources no longer
// listed are closed and dropped. Paths that fail to open are reported in the
// returned error, but every path that did open successfully is still kept.
func (l *Library) SetSourcesJSON(pathsJSON string) error {
	var sources librarySources
	if err := json.Unmarshal([]byte(pathsJSON), &sources); err != nil {
		return fmt.Errorf("parsing library sources: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var failures []string

	newCollections := make(map[string]*collection.Collection, len(sources.Collections))
	for _, p := range sources.Collections {
		if col, ok := l.collections[p]; ok {
			newCollections[p] = col
			continue
		}

		col, err := collection.OpenReadOnly(p)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", p, err))
			continue
		}
		newCollections[p] = col
	}
	for p, col := range l.collections {
		if _, stillWanted := newCollections[p]; !stillWanted {
			col.Close()
		}
	}
	l.collections = newCollections

	newFiles := make(map[string]*CardFile, len(sources.Cards))
	for _, p := range sources.Cards {
		if cf, ok := l.files[p]; ok {
			newFiles[p] = cf
			continue
		}

		cf, err := OpenCardFile(p)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", p, err))
			continue
		}
		newFiles[p] = cf
	}
	l.files = newFiles

	if len(failures) > 0 {
		return fmt.Errorf("couldn't open %d source(s): %s", len(failures), strings.Join(failures, "; "))
	}
	return nil
}

// Close closes every open collection and clears the Library's sources.
func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var firstErr error
	for _, col := range l.collections {
		if err := col.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	l.collections = make(map[string]*collection.Collection)
	l.files = make(map[string]*CardFile)

	return firstErr
}

// libraryHit is one search result: the source path it came from (a
// collection file or a bare card file), its summary, and a text snippet.
type libraryHit struct {
	Source  string                `json:"source"`
	Card    collection.CardSummary `json:"card"`
	Snippet string                `json:"snippet"`
}

// SearchJSON searches every open collection's FTS index (serially) and
// case-insensitive substring-matches every cached bare file's searchable
// text, returning a JSON array of libraryHit. Collection hits come first, in
// each collection's own rank order; bare-file hits are appended after.
func (l *Library) SearchJSON(query string) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var hits []libraryHit

	for _, p := range sortedKeys(l.collections) {
		results, err := l.collections[p].Search(query)
		if err != nil {
			continue
		}
		for _, r := range results {
			hits = append(hits, libraryHit{Source: p, Card: r.CardSummary, Snippet: r.Snippet})
		}
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	if needle != "" {
		for _, p := range sortedFileKeys(l.files) {
			cf := l.files[p]
			if !strings.Contains(strings.ToLower(cf.searchableText()), needle) {
				continue
			}
			hits = append(hits, libraryHit{Source: p, Card: cf.summary(), Snippet: cf.name})
		}
	}

	return marshalJSONArray(hits)
}

// SearchFilteredJSON decodes a collection.SearchFilter from filterJSON and
// runs a field-scoped search across every open collection (via
// collection.SearchFiltered) and bare card file (via CardFile.matchesFilter),
// returning a JSON array of libraryHit in the same order as SearchJSON:
// collection hits first (in each collection's own rank order), then
// bare-file hits.
func (l *Library) SearchFilteredJSON(filterJSON string) (string, error) {
	var filter collection.SearchFilter
	if err := json.Unmarshal([]byte(filterJSON), &filter); err != nil {
		return "", fmt.Errorf("parsing search filter: %w", err)
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	var hits []libraryHit

	for _, p := range sortedKeys(l.collections) {
		results, err := l.collections[p].SearchFiltered(filter)
		if err != nil {
			continue
		}
		for _, r := range results {
			hits = append(hits, libraryHit{Source: p, Card: r.CardSummary, Snippet: r.Snippet})
		}
	}

	for _, p := range sortedFileKeys(l.files) {
		cf := l.files[p]
		if !cf.matchesFilter(filter) {
			continue
		}
		hits = append(hits, libraryHit{Source: p, Card: cf.summary(), Snippet: cf.name})
	}

	return marshalJSONArray(hits)
}

// personKey identifies a collection.PersonRef for deduplication across
// sources: the same name with a different uri (or vice versa) is a different
// person, mirroring collection.People()'s merge rule.
type personKey struct {
	name string
	uri  string
}

// personRoleOrder fixes the order roles are appended in when merging, so
// PersonRef.Roles is deterministic regardless of source iteration order.
var personRoleOrder = []string{"from", "to", "collector"}

// mergePersonRole folds one (name, uri, role) observation into dedup,
// skipping rows where both name and uri are empty, and merging Roles when
// the same (name, uri) pair recurs across sources.
func mergePersonRole(dedup map[personKey]*collection.PersonRef, name, uri, role string) {
	if name == "" && uri == "" {
		return
	}

	key := personKey{name: name, uri: uri}
	ref, ok := dedup[key]
	if !ok {
		ref = &collection.PersonRef{Name: name, Uri: uri}
		dedup[key] = ref
	}

	for _, existing := range ref.Roles {
		if existing == role {
			return
		}
	}
	ref.Roles = append(ref.Roles, role)
	sort.Slice(ref.Roles, func(i, j int) bool {
		return personRoleIndex(ref.Roles[i]) < personRoleIndex(ref.Roles[j])
	})
}

func personRoleIndex(role string) int {
	for i, r := range personRoleOrder {
		if r == role {
			return i
		}
	}
	return len(personRoleOrder)
}

// PeopleJSON returns the union of distinct people across every open
// collection and every open bare card file, as a JSON array of
// collection.PersonRef.
func (l *Library) PeopleJSON() (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	dedup := make(map[personKey]*collection.PersonRef)

	for _, p := range sortedKeys(l.collections) {
		people, err := l.collections[p].People()
		if err != nil {
			continue
		}
		for _, person := range people {
			for _, role := range person.Roles {
				mergePersonRole(dedup, person.Name, person.Uri, role)
			}
		}
	}

	for _, p := range sortedFileKeys(l.files) {
		cf := l.files[p]
		mergePersonRole(dedup, cf.meta.Sender.Name, cf.meta.Sender.Uri, "from")
		mergePersonRole(dedup, cf.meta.Recipient.Name, cf.meta.Recipient.Uri, "to")
		mergePersonRole(dedup, cf.meta.Context.Author.Name, cf.meta.Context.Author.Uri, "collector")
	}

	// personKey uniquely determines (Name, Uri), so sorting by those fields
	// alone is enough for deterministic output.
	people := make([]collection.PersonRef, 0, len(dedup))
	for _, ref := range dedup {
		people = append(people, *ref)
	}
	sort.Slice(people, func(i, j int) bool {
		if people[i].Name != people[j].Name {
			return people[i].Name < people[j].Name
		}
		return people[i].Uri < people[j].Uri
	})

	return marshalJSONArray(people)
}

func sortedKeys(m map[string]*collection.Collection) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedFileKeys(m map[string]*CardFile) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
