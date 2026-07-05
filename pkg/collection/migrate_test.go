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
