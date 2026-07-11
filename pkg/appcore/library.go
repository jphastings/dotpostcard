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
