// Package testutil provides helpers for setting up test infrastructure.
package testutil

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/hastenr/chatapi/internal/db"
	_ "github.com/mattn/go-sqlite3"
)

var testDBCounter atomic.Int64

// NewTestDB returns a fully migrated in-memory SQLite database.
// The connection is closed automatically when the test ends.
//
// Uses a named shared-cache in-memory database so that multiple connections
// within the same test all share the same schema and data, avoiding the
// per-connection isolation of plain ":memory:" SQLite databases.
func NewTestDB(t *testing.T) *db.DB {
	t.Helper()

	// Each test gets a unique DB name so they don't interfere with each other.
	id := testDBCounter.Add(1)
	dsn := fmt.Sprintf("file:testdb_%d?mode=memory&cache=shared&_busy_timeout=5000", id)

	database, err := db.New(dsn)
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
