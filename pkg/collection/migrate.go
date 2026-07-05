package collection

import (
	"database/sql"
	"fmt"
)

// migrations is the schema upgrade ladder: migrations[i] upgrades a
// collection from schema version i+1 to version i+2. The current schema
// version is derived from the ladder (see schemaVersion), so adding a
// migration is just appending a step here — no version constant to bump. The
// runner stamps PRAGMA user_version and meta.schema_version itself after
// each step, so a migration func only alters schema/data. When appending a
// step, also update schemaStatements in schema.go to describe the resulting
// schema: newly-Created collections are built directly at the current
// version and never run this ladder.
var migrations = []func(*sql.Tx) error{
	// v1 -> v2: add an optional, user-editable collection title.
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE meta ADD COLUMN title TEXT`)
		return err
	},
}

func schemaVersion() int { return 1 + len(migrations) }

// ensureSchema errors unless db is a postcard collection at the current
// schema version — except when an out-of-date collection is opened
// read-write, in which case it's migrated up to the current version first.
func ensureSchema(db *sql.DB, readOnly bool) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	switch {
	case version == schemaVersion():
		return nil
	case version > schemaVersion():
		return fmt.Errorf("collection uses schema version %d, but this build of postcards only supports up to %d; the collection was created by a newer version of postcards — update the app", version, schemaVersion())
	case version < 1:
		return fmt.Errorf("not a postcard collection (found schema version %d)", version)
	case readOnly:
		return fmt.Errorf("collection uses outdated schema version %d (current is %d) and can't be migrated while open read-only; upgrade this collection with the postcards CLI", version, schemaVersion())
	default:
		return migrate(db, version)
	}
}

// migrate runs each pending migration in its own transaction, stamping both
// version markers before committing, so a failed step rolls back cleanly and
// leaves the collection at the last fully-applied version.
func migrate(db *sql.DB, from int) error {
	for v := from; v < schemaVersion(); v++ {
		if err := migrateStep(db, v); err != nil {
			return fmt.Errorf("migrating collection from schema version %d to %d: %w", v, v+1, err)
		}
	}
	return nil
}

func migrateStep(db *sql.DB, from int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := migrations[from-1](tx); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, from+1)); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE meta SET schema_version = ?`, from+1); err != nil {
		return err
	}

	return tx.Commit()
}
