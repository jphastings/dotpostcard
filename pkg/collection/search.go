package collection

import (
	"fmt"
	"strings"
)

const qualifiedSummaryColumns = `cards.name, cards.filename, cards.mimetype, cards.flip, cards.sent_on, cards.sender_name, cards.recipient_name, cards.location_name, cards.country_code, cards.latitude, cards.longitude, cards.front_px_w, cards.front_px_h`

// SearchResult is a CardSummary plus the full-text-search context: a
// highlighted snippet of the matching text, and its bm25 rank (lower is
// a better match).
type SearchResult struct {
	CardSummary
	Snippet string  `json:"snippet"`
	Rank    float64 `json:"rank"`
}

// Search runs a full-text search across card names, sender/recipient/location
// names, descriptions, transcripts and context, best matches first.
// The query is free text; it's safe to pass arbitrary user input.
func (c *Collection) Search(query string) ([]SearchResult, error) {
	ftsQuery := sanitizeFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}

	rows, err := c.db.Query(`
		SELECT `+qualifiedSummaryColumns+`, snippet(cards_fts, -1, '<b>', '</b>', '…', 12), bm25(cards_fts) AS rank
		FROM cards_fts
		JOIN cards ON cards.id = cards_fts.rowid
		WHERE cards_fts MATCH ?
		ORDER BY rank
	`, ftsQuery)
	if err != nil {
		return nil, fmt.Errorf("searching %q: %w", query, err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var s summaryScan
		var snippet string
		var rank float64

		if err := rows.Scan(append(s.dest(), &snippet, &rank)...); err != nil {
			return nil, fmt.Errorf("searching %q: %w", query, err)
		}

		results = append(results, SearchResult{CardSummary: s.result(), Snippet: snippet, Rank: rank})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("searching %q: %w", query, err)
	}

	return results, nil
}

// sanitizeFTSQuery turns free text into a safe FTS5 MATCH expression: every
// whitespace-separated term becomes an independently-matched (implicit AND),
// quoted, prefix-matched phrase, so FTS5 query-syntax characters in user
// input (", *, (, ) etc.) can never cause a MATCH syntax error.
func sanitizeFTSQuery(query string) string {
	fields := strings.Fields(query)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		terms = append(terms, fmt.Sprintf(`"%s"*`, strings.ReplaceAll(f, `"`, `""`)))
	}
	return strings.Join(terms, " ")
}
