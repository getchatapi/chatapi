package webhook_test

// Blank import of go-sqlite3 forces CGO linking, which adds the LC_UUID load
// command required by macOS 26.x. Pure-Go test binaries crash without it.
import _ "github.com/mattn/go-sqlite3"
