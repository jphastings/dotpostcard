package collection

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jphastings/dotpostcard/internal/sqldb"
)

func backfillTitles(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, name, metadata_json FROM cards`)
	if err != nil {
		return err
	}
	titles := map[int64]string{}
	for rows.Next() {
		var id int64
		var name, metadataJSON string
		if err := rows.Scan(&id, &name, &metadataJSON); err != nil {
			rows.Close()
			return err
		}
		var sm storedMetadata
		if err := json.Unmarshal([]byte(metadataJSON), &sm); err != nil {
			rows.Close()
			return fmt.Errorf("parsing metadata for %q: %w", name, err)
		}
		titles[id] = sm.toMetadata().EffectiveTitle(name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	for id, title := range titles {
		if _, err := tx.Exec(`UPDATE cards SET title = ? WHERE id = ?`, title, id); err != nil {
			return err
		}
	}
	return nil
}

// migrations is the schema upgrade ladder: migrations[i] upgrades a
// collection from schema version i+1 to version i+2. The current schema
// version is derived from the ladder (see schemaVersion), so adding a
// migration is just appending a step here — no version constant to bump. The
// runner runs the whole ladder in one transaction and stamps PRAGMA
// user_version and meta.schema_version itself, so a migration func only
// alters schema/data. Migration is lossless — card images are stored
// untouched (no re-encoding, no quality loss) and all metadata is preserved —
// and every new step must keep that guarantee. When appending a
// step, also update schemaStatements in schema.go to describe the resulting
// schema: newly-Created collections are built directly at the current
// version and never run this ladder.
var migrations = []func(*sql.Tx) error{
	// v1 -> v2: add an optional, user-editable collection title.
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE meta ADD COLUMN title TEXT`)
		return err
	},
	// v2 -> v3: give each card a title (backfilled with its effective title), searchable
	// through the full-text index, and let the collection remember where it was published.
	func(tx *sql.Tx) error {
		for _, stmt := range []string{
			`DROP TRIGGER cards_ai`,
			`DROP TRIGGER cards_ad`,
			`DROP TRIGGER cards_au`,
			`DROP TABLE cards_fts`,
			`ALTER TABLE cards ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE meta ADD COLUMN published_uri TEXT`,
		} {
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		}

		if err := backfillTitles(tx); err != nil {
			return err
		}

		for _, stmt := range ftsStatements() {
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO cards_fts(cards_fts) VALUES('rebuild')`)
		return err
	},
}

func schemaVersion() int { return 1 + len(migrations) }

// SchemaStatus says how a collection file's schema compares with this build's.
type SchemaStatus int

const (
	SchemaCurrent  SchemaStatus = iota // readable and writable as-is
	SchemaOutdated                     // written by an older postcards; Migrate upgrades it
	SchemaNewer                        // written by a newer postcards; this build can't open it
)

var (
	// ErrOutdatedSchema is wrapped by the error OpenReadOnly returns for an outdated collection.
	ErrOutdatedSchema = errors.New("collection schema is outdated")
	// ErrNewerSchema is wrapped by the error Open and OpenReadOnly return for a collection from a newer postcards.
	ErrNewerSchema = errors.New("collection schema is newer than this build supports")
)

func classifyVersion(version int) (SchemaStatus, error) {
	switch {
	case version < 1:
		return 0, fmt.Errorf("not a postcard collection (found schema version %d)", version)
	case version > schemaVersion():
		return SchemaNewer, nil
	case version < schemaVersion():
		return SchemaOutdated, nil
	default:
		return SchemaCurrent, nil
	}
}

func readVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return version, nil
}

// Status reports whether the collection at path is at the current schema,
// needs Migrate, or is from a newer postcards. It only reads the version
// stamp, so it's cheap and never modifies the file.
func Status(path string) (SchemaStatus, error) {
	db, err := sqldb.Open(path, true)
	if err != nil {
		return 0, fmt.Errorf("opening collection at %s: %w", path, err)
	}
	defer db.Close()

	version, err := readVersion(db)
	if err != nil {
		return 0, fmt.Errorf("checking collection at %s: %w", path, err)
	}
	status, err := classifyVersion(version)
	if err != nil {
		return 0, fmt.Errorf("checking collection at %s: %w", path, err)
	}
	return status, nil
}

// Migrate upgrades the collection at path to the current schema, all or
// nothing, and does nothing if it's already current. The upgrade is lossless:
// card images are stored untouched (no re-encoding, no quality loss) and all
// metadata is preserved. It errors, wrapping ErrNewerSchema, for a collection
// from a newer postcards.
func Migrate(path string) error {
	col, err := Open(path)
	if err != nil {
		return err
	}
	return col.Close()
}

// ensureSchema errors unless db is a postcard collection at the current
// schema version — except when an out-of-date collection is opened
// read-write, in which case it's migrated up to the current version first.
func ensureSchema(db *sql.DB, readOnly bool) error {
	version, err := readVersion(db)
	if err != nil {
		return err
	}
	status, err := classifyVersion(version)
	if err != nil {
		return err
	}

	switch {
	case status == SchemaCurrent:
		return nil
	case status == SchemaNewer:
		return fmt.Errorf("collection uses schema version %d, but this build of postcards only supports up to %d; the collection was created by a newer version of postcards — update the app: %w", version, schemaVersion(), ErrNewerSchema)
	case readOnly:
		return fmt.Errorf("collection uses outdated schema version %d (current is %d) and can't be migrated while open read-only; upgrade this collection with the postcards CLI: %w", version, schemaVersion(), ErrOutdatedSchema)
	default:
		return migrate(db, version)
	}
}

// migrate runs every pending migration in a single transaction, stamping both
// version markers once at the end, so a failure at any step rolls everything
// back and leaves the file exactly as it was — never half-migrated, even if
// it's synced mid-way.
func migrate(db *sql.DB, from int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for v := from; v < schemaVersion(); v++ {
		if err := migrations[v-1](tx); err != nil {
			return fmt.Errorf("migrating collection from schema version %d to %d: %w", v, v+1, err)
		}
	}

	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion())); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE meta SET schema_version = ?`, schemaVersion()); err != nil {
		return err
	}

	return tx.Commit()
}
