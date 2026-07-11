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

// SearchFilter is a structured, field-scoped search, built by the app from a
// user query like "from:Claire country:ESP beach". Every populated field
// narrows the results; multiple values within one field are OR'd together
// (any one of them may match), while different fields are AND'd (every
// populated field must be satisfied).
type SearchFilter struct {
	// Text holds free-text terms, matched the same way as Search.
	Text string `json:"text,omitempty"`
	// From matches the sender: each value is either a person's URI (exact
	// match) or a name (FTS prefix match).
	From []string `json:"from,omitempty"`
	// To matches the recipient, the same way as From.
	To []string `json:"to,omitempty"`
	// With matches either the sender or the recipient (but never the
	// Collector/context author), the same way as From/To.
	With []string `json:"with,omitempty"`
	// Collector matches the "catalogued/collected by" context author, the
	// same way as From/To.
	Collector []string `json:"collector,omitempty"`
	// Country restricts to cards whose location country code (ISO 3166-1
	// alpha-3, uppercase) is one of these values. The app is responsible for
	// normalising to that form before calling in.
	Country []string `json:"country,omitempty"`
	// SentFrom is an inclusive lower bound (ISO "yyyy-MM-dd") on sent_on.
	SentFrom string `json:"sent_from,omitempty"`
	// SentUntil is an exclusive upper bound (ISO "yyyy-MM-dd") on sent_on.
	SentUntil string `json:"sent_until,omitempty"`
}

// IsPersonURI reports whether a From/To/With/Collector filter value
// identifies a person by URI (exact match) rather than by name (prefix
// match): a URI contains "://" or has a "mailto:" prefix.
func IsPersonURI(v string) bool {
	return strings.Contains(v, "://") || strings.HasPrefix(v, "mailto:")
}

// splitPersonValues separates a From/To/With/Collector value list into
// name-form and URI-form values.
func splitPersonValues(values []string) (names, uris []string) {
	for _, v := range values {
		if IsPersonURI(v) {
			uris = append(uris, v)
		} else {
			names = append(names, v)
		}
	}
	return names, uris
}

// personNameMatchFragment builds an FTS5 expression matching any of names as
// a quoted, prefix-matched phrase, scoped to column (a single FTS column, or
// an FTS5 "{col1 col2}" column-set). Embedded double-quotes are escaped the
// same way sanitizeFTSQuery escapes them, but each value is kept as one
// phrase (not split into independently-matched words) since it names one
// person. Multiple values are OR'd together.
func personNameMatchFragment(column string, names []string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf(`%s : "%s"*`, column, strings.ReplaceAll(n, `"`, `""`))
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// uriWhereFragment builds a WHERE-clause expression matching any of uris
// against one or more json_extract'd metadata paths (OR'd across paths, e.g.
// With's sender-or-recipient), appending each uri's placeholder argument to
// args in the same order it appears in the returned SQL text.
func uriWhereFragment(jsonPaths []string, uris []string, args *[]any) string {
	clauses := make([]string, len(jsonPaths))
	for pi, path := range jsonPaths {
		placeholders := make([]string, len(uris))
		for i, u := range uris {
			placeholders[i] = "?"
			*args = append(*args, u)
		}
		clauses[pi] = fmt.Sprintf(`json_extract(cards.metadata_json, '%s') IN (%s)`, path, strings.Join(placeholders, ", "))
	}
	if len(clauses) == 1 {
		return clauses[0]
	}
	return "(" + strings.Join(clauses, " OR ") + ")"
}

// SearchFiltered runs a field-scoped search; see SearchFilter's doc comment
// for its OR/AND semantics. Search's free-text behaviour is unchanged and
// available here as an empty SearchFilter{Text: query}.
//
// URI values are matched with json_extract(metadata_json, ...) rather than a
// Go-side post-filter: both SQLite drivers this codebase builds against
// (modernc.org/sqlite for desktop/test builds, github.com/mattn/go-sqlite3
// for the gomobile build) bundle SQLite >= 3.38, where the JSON functions
// (including json_extract) are compiled into the core build unconditionally
// (SQLITE_OMIT_JSON is unset in both), so json_extract is always available.
//
// Name filters are FTS5 column-filter expressions (e.g. `sender_name :
// "claire"*`) folded into the same MATCH expression as Text, so they still
// benefit from bm25 ranking; a field that mixes name and URI values (e.g.
// From: ["Claire", "https://alice.example.com"]) instead becomes a WHERE
// clause OR of a `cards.id IN (SELECT rowid FROM cards_fts WHERE cards_fts
// MATCH ...)` subquery and the URI check, since FTS5's MATCH operator can't
// appear inside a plain WHERE clause's OR expression directly.
func (c *Collection) SearchFiltered(filter SearchFilter) ([]SearchResult, error) {
	fromNames, fromURIs := splitPersonValues(filter.From)
	toNames, toURIs := splitPersonValues(filter.To)
	withNames, withURIs := splitPersonValues(filter.With)
	collectorNames, collectorURIs := splitPersonValues(filter.Collector)

	var matchParts, whereParts []string
	var args []any

	if text := sanitizeFTSQuery(filter.Text); text != "" {
		matchParts = append(matchParts, text)
	}

	addPersonField := func(column string, jsonPaths []string, names, uris []string) {
		switch {
		case len(names) == 0 && len(uris) == 0:
			return
		case len(uris) == 0:
			matchParts = append(matchParts, personNameMatchFragment(column, names))
		case len(names) == 0:
			whereParts = append(whereParts, uriWhereFragment(jsonPaths, uris, &args))
		default:
			args = append(args, personNameMatchFragment(column, names))
			uriExpr := uriWhereFragment(jsonPaths, uris, &args)
			whereParts = append(whereParts, "(cards.id IN (SELECT rowid FROM cards_fts WHERE cards_fts MATCH ?) OR "+uriExpr+")")
		}
	}

	addPersonField("sender_name", []string{"$.sender.uri"}, fromNames, fromURIs)
	addPersonField("recipient_name", []string{"$.recipient.uri"}, toNames, toURIs)
	addPersonField("{sender_name recipient_name}", []string{"$.sender.uri", "$.recipient.uri"}, withNames, withURIs)
	addPersonField("context_author_name", []string{"$.context.author.uri"}, collectorNames, collectorURIs)

	if len(filter.Country) > 0 {
		placeholders := make([]string, len(filter.Country))
		for i, cc := range filter.Country {
			placeholders[i] = "?"
			args = append(args, cc)
		}
		whereParts = append(whereParts, fmt.Sprintf("cards.country_code IN (%s)", strings.Join(placeholders, ", ")))
	}

	// sent_on is compared as text against ISO "yyyy-MM-dd" bounds, which
	// sorts identically to a date comparison; a NULL/undated sent_on
	// compares to NULL (never true) against either bound, so undated cards
	// are correctly excluded whenever a bound is given, with no extra check.
	if filter.SentFrom != "" {
		whereParts = append(whereParts, "cards.sent_on >= ?")
		args = append(args, filter.SentFrom)
	}
	if filter.SentUntil != "" {
		whereParts = append(whereParts, "cards.sent_on < ?")
		args = append(args, filter.SentUntil)
	}

	if len(matchParts) == 0 {
		return c.searchFilteredPlain(whereParts, args)
	}
	return c.searchFilteredFTS(strings.Join(matchParts, " "), whereParts, args)
}

func (c *Collection) searchFilteredFTS(matchQuery string, whereParts []string, whereArgs []any) ([]SearchResult, error) {
	query := `
		SELECT ` + qualifiedSummaryColumns + `, snippet(cards_fts, -1, '<b>', '</b>', '…', 12), bm25(cards_fts) AS rank
		FROM cards_fts
		JOIN cards ON cards.id = cards_fts.rowid
		WHERE cards_fts MATCH ?`
	for _, w := range whereParts {
		query += " AND " + w
	}
	query += " ORDER BY rank"

	args := append([]any{matchQuery}, whereArgs...)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var s summaryScan
		var snippet string
		var rank float64

		if err := rows.Scan(append(s.dest(), &snippet, &rank)...); err != nil {
			return nil, fmt.Errorf("searching: %w", err)
		}

		results = append(results, SearchResult{CardSummary: s.result(), Snippet: snippet, Rank: rank})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}

	return results, nil
}

func (c *Collection) searchFilteredPlain(whereParts []string, args []any) ([]SearchResult, error) {
	query := `SELECT ` + qualifiedSummaryColumns + ` FROM cards`
	if len(whereParts) > 0 {
		query += " WHERE " + strings.Join(whereParts, " AND ")
	}
	query += " ORDER BY sent_on DESC NULLS LAST, name ASC"

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		summary, err := scanSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("searching: %w", err)
		}
		results = append(results, SearchResult{CardSummary: summary})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}

	return results, nil
}
