package collection

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jphastings/dotpostcard/internal/sqldb"
	"github.com/stretchr/testify/assert"
)

// withTestMigration appends a step to the migration ladder, so the collection
// created before the append is one schema version behind for the rest of the test.
func withTestMigration(t *testing.T, step func(*sql.Tx) error) {
	t.Helper()
	migrations = append(migrations, step)
	t.Cleanup(func() { migrations = migrations[:len(migrations)-1] })
}

func createOutdatedCollection(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.postcards")

	col, err := Create(path)
	assert.NoError(t, err)
	assert.NoError(t, col.Close())

	return path
}

func fileVersions(t *testing.T, path string) (userVersion, metaVersion int) {
	t.Helper()

	db, err := sqldb.Open(path, true)
	assert.NoError(t, err)
	defer db.Close()

	assert.NoError(t, db.QueryRow(`PRAGMA user_version`).Scan(&userVersion))
	assert.NoError(t, db.QueryRow(`SELECT schema_version FROM meta`).Scan(&metaVersion))
	return
}

func TestOpenMigratesOutdatedCollections(t *testing.T) {
	path := createOutdatedCollection(t)
	oldVersion := schemaVersion()

	withTestMigration(t, func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE cards ADD COLUMN test_migration_ran INTEGER`)
		return err
	})

	col, err := Open(path)
	assert.NoError(t, err)
	assert.NoError(t, col.Close())

	userVersion, metaVersion := fileVersions(t, path)
	assert.Equal(t, oldVersion+1, userVersion)
	assert.Equal(t, oldVersion+1, metaVersion)
}

func TestFailedMigrationRollsBack(t *testing.T) {
	path := createOutdatedCollection(t)
	oldVersion := schemaVersion()

	withTestMigration(t, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE cards ADD COLUMN should_be_rolled_back INTEGER`); err != nil {
			return err
		}
		return errors.New("deliberate migration failure")
	})

	_, err := Open(path)
	assert.ErrorContains(t, err, "deliberate migration failure")

	userVersion, metaVersion := fileVersions(t, path)
	assert.Equal(t, oldVersion, userVersion, "a failed migration should leave the version stamp untouched")
	assert.Equal(t, oldVersion, metaVersion)

	db, err := sqldb.Open(path, true)
	assert.NoError(t, err)
	defer db.Close()
	var columnExists int
	assert.NoError(t, db.QueryRow(`SELECT count(*) FROM pragma_table_info('cards') WHERE name = 'should_be_rolled_back'`).Scan(&columnExists))
	assert.Zero(t, columnExists, "schema changes from a failed migration should be rolled back")
}

func TestOpenReadOnlyNeverMigrates(t *testing.T) {
	path := createOutdatedCollection(t)
	oldVersion := schemaVersion()

	withTestMigration(t, func(tx *sql.Tx) error { return nil })

	_, err := OpenReadOnly(path)
	assert.ErrorContains(t, err, "postcards CLI")

	userVersion, _ := fileVersions(t, path)
	assert.Equal(t, oldVersion, userVersion)
}

// schemaStatementsV1 is a frozen copy of schemaStatements as it stood before
// the title column (schema v2) was added, kept only so this test can build a
// real v1 collection file to migrate.
var schemaStatementsV1 = []string{
	`CREATE TABLE meta (
		schema_version INTEGER NOT NULL,
		created_by TEXT NOT NULL
	)`,
	`CREATE TABLE cards (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
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
	`CREATE VIRTUAL TABLE cards_fts USING fts5(
		` + ftsColumns + `,
		content='cards',
		content_rowid='id',
		tokenize='unicode61 remove_diacritics 2'
	)`,
	`CREATE TRIGGER cards_ai AFTER INSERT ON cards BEGIN
		INSERT INTO cards_fts(rowid, ` + ftsColumns + `)
		VALUES (new.id, new.name, new.sender_name, new.recipient_name, new.location_name, new.front_description, new.back_description, new.front_transcript, new.back_transcript, new.context_description, new.context_author_name);
	END`,
	`CREATE TRIGGER cards_ad AFTER DELETE ON cards BEGIN
		INSERT INTO cards_fts(cards_fts, rowid, ` + ftsColumns + `)
		VALUES ('delete', old.id, old.name, old.sender_name, old.recipient_name, old.location_name, old.front_description, old.back_description, old.front_transcript, old.back_transcript, old.context_description, old.context_author_name);
	END`,
	`CREATE TRIGGER cards_au AFTER UPDATE ON cards BEGIN
		INSERT INTO cards_fts(cards_fts, rowid, ` + ftsColumns + `)
		VALUES ('delete', old.id, old.name, old.sender_name, old.recipient_name, old.location_name, old.front_description, old.back_description, old.front_transcript, old.back_transcript, old.context_description, old.context_author_name);
		INSERT INTO cards_fts(rowid, ` + ftsColumns + `)
		VALUES (new.id, new.name, new.sender_name, new.recipient_name, new.location_name, new.front_description, new.back_description, new.front_transcript, new.back_transcript, new.context_description, new.context_author_name);
	END`,
}

// createV1Collection builds a real schema-v1 collection file (predating the
// title column) by executing the frozen v1 DDL directly, so the migration
// test below exercises the actual v1->v2 step rather than a synthetic stand-in.
func createV1Collection(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "v1.postcards")

	db, err := sqldb.Open(path, false)
	assert.NoError(t, err)
	defer db.Close()

	for _, stmt := range schemaStatementsV1 {
		_, err := db.Exec(stmt)
		assert.NoError(t, err)
	}
	_, err = db.Exec(`PRAGMA user_version = 1`)
	assert.NoError(t, err)
	_, err = db.Exec(`INSERT INTO meta (schema_version, created_by) VALUES (1, 'test-fixture')`)
	assert.NoError(t, err)

	return path
}

func TestOpenMigratesRealV1CollectionAddsTitleColumn(t *testing.T) {
	path := createV1Collection(t)

	col, err := Open(path)
	assert.NoError(t, err)
	defer col.Close()

	userVersion, metaVersion := fileVersions(t, path)
	assert.Equal(t, 2, userVersion)
	assert.Equal(t, 2, metaVersion)

	title, err := col.Title()
	assert.NoError(t, err)
	assert.Empty(t, title)

	assert.NoError(t, col.SetTitle("My Collection"))
	title, err = col.Title()
	assert.NoError(t, err)
	assert.Equal(t, "My Collection", title)
}

func TestOpenRejectsNewerCollections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.postcards")

	col, err := Create(path)
	assert.NoError(t, err)
	_, err = col.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion()+1))
	assert.NoError(t, err)
	assert.NoError(t, col.Close())

	_, err = Open(path)
	assert.ErrorContains(t, err, "newer version of postcards")

	_, err = OpenReadOnly(path)
	assert.ErrorContains(t, err, "newer version of postcards")
}
