package collection

import (
	"database/sql"
	"fmt"
	"strings"
)

const ftsColumns = `name, title, sender_name, recipient_name, location_name, front_description, back_description, front_transcript, back_transcript, context_description, context_author_name`

var schemaStatements = []string{
	`CREATE TABLE meta (
		schema_version INTEGER NOT NULL,
		created_by TEXT NOT NULL,
		title TEXT,
		published_uri TEXT
	)`,
	`CREATE TABLE cards (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL DEFAULT '',
		filename TEXT NOT NULL,
		mimetype TEXT NOT NULL,
		data BLOB NOT NULL,
		file_sha256 TEXT NOT NULL,
		thumb BLOB,
		metadata_json TEXT NOT NULL,
		flip TEXT,
		sent_on TEXT,
		locale TEXT,
		sender_name TEXT,
		recipient_name TEXT,
		location_name TEXT,
		country_code TEXT,
		latitude REAL,
		longitude REAL,
		front_px_w INTEGER,
		front_px_h INTEGER,
		front_description TEXT,
		back_description TEXT,
		front_transcript TEXT,
		back_transcript TEXT,
		context_description TEXT,
		context_author_name TEXT,
		added_at TEXT NOT NULL
	)`,
}

// ftsStatements builds the cards_fts index and the triggers that keep it in step with cards.
func ftsStatements() []string {
	newCols := "new." + strings.ReplaceAll(ftsColumns, ", ", ", new.")
	oldCols := "old." + strings.ReplaceAll(ftsColumns, ", ", ", old.")
	return []string{
		`CREATE VIRTUAL TABLE cards_fts USING fts5(
		` + ftsColumns + `,
		content='cards',
		content_rowid='id',
		tokenize='unicode61 remove_diacritics 2'
	)`,
		`CREATE TRIGGER cards_ai AFTER INSERT ON cards BEGIN
		INSERT INTO cards_fts(rowid, ` + ftsColumns + `)
		VALUES (new.id, ` + newCols + `);
	END`,
		`CREATE TRIGGER cards_ad AFTER DELETE ON cards BEGIN
		INSERT INTO cards_fts(cards_fts, rowid, ` + ftsColumns + `)
		VALUES ('delete', old.id, ` + oldCols + `);
	END`,
		`CREATE TRIGGER cards_au AFTER UPDATE ON cards BEGIN
		INSERT INTO cards_fts(cards_fts, rowid, ` + ftsColumns + `)
		VALUES ('delete', old.id, ` + oldCols + `);
		INSERT INTO cards_fts(rowid, ` + ftsColumns + `)
		VALUES (new.id, ` + newCols + `);
	END`,
	}
}

// initSchema creates a fresh, current-version schema in an empty database
// file. The version is stored both in `PRAGMA user_version` (so any sqlite
// tool can see it) and in the `meta` table (cheap redundancy that makes
// `meta` self-describing without needing a pragma).
func initSchema(db *sql.DB, createdBy string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("starting schema transaction: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range append(schemaStatements, ftsStatements()...) {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("creating schema: %w", err)
		}
	}

	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion())); err != nil {
		return fmt.Errorf("setting schema version: %w", err)
	}

	if _, err := tx.Exec(`INSERT INTO meta (schema_version, created_by) VALUES (?, ?)`, schemaVersion(), createdBy); err != nil {
		return fmt.Errorf("recording collection metadata: %w", err)
	}

	return tx.Commit()
}
