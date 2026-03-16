// Package testutil provides helpers for setting up test infrastructure.
package testutil

import (
	"testing"

	"github.com/hastenr/chatapi/internal/db"
	_ "github.com/mattn/go-sqlite3"
)

// NewTestDB returns a fully migrated in-memory SQLite database.
// The connection is closed automatically when the test ends.
func NewTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("testutil.NewTestDB: open: %v", err)
	}

	if err := db.RunMigrations(database); err != nil {
		database.Close()
		t.Fatalf("testutil.NewTestDB: migrations: %v", err)
	}

	t.Cleanup(func() { database.Close() })

	return database
}
