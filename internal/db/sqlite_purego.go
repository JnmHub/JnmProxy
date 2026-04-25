//go:build sqlite_purego

package db

import _ "modernc.org/sqlite"

func sqliteDriverName() string {
	return "sqlite"
}
