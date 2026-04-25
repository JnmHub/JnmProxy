//go:build !sqlite_purego

package db

import _ "github.com/mattn/go-sqlite3"

func sqliteDriverName() string {
	return "sqlite3"
}
