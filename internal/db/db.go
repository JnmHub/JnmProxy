package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("create database directory %q: %w", dir, err)
			}
		}
	}

	dsn := fmt.Sprintf("file:%s", path)
	store, err := sql.Open(sqliteDriverName(), dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store.SetMaxOpenConns(1)

	if err := store.PingContext(ctx); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := configureSQLite(ctx, store, path); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func configureSQLite(ctx context.Context, store *sql.DB, path string) error {
	if _, err := store.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := store.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if path != ":memory:" {
		if _, err := store.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
			return fmt.Errorf("set sqlite journal mode: %w", err)
		}
	}
	return nil
}
