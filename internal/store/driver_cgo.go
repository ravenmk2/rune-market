//go:build cgo

package store

// CGO builds keep using the pure-Go modernc driver so the dependency set and
// behavior stay identical to the CGO_ENABLED=0 baseline; swap this file for a
// CGO sqlite3 driver if one is ever introduced.
import _ "modernc.org/sqlite"

const sqliteDriverName = "sqlite"
