package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateCreatesCoreTables(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if err := Migrate(ctx, store); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if err := Migrate(ctx, store); err != nil {
		t.Fatalf("second Migrate returned error: %v", err)
	}

	for _, table := range []string{
		"subscriptions",
		"subscription_refresh_logs",
		"proxy_nodes",
		"proxy_groups",
		"proxy_node_groups",
		"group_keywords",
		"credentials",
		"credential_bindings",
		"traffic_stats_hourly",
		"traffic_stats_daily",
		"node_health_checks",
		"system_settings",
		"sing_box_engine_states",
		"operation_logs",
	} {
		if !tableExists(t, store, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	for _, column := range []string{
		"sing_box_outbound_json",
		"sing_box_status",
		"sing_box_error",
		"sing_box_version",
		"udp_supported",
		"transport_type",
	} {
		if !columnExists(t, store, "proxy_nodes", column) {
			t.Fatalf("expected proxy_nodes.%s to exist", column)
		}
	}
	for _, column := range []string{
		"sing_box_supported_count",
		"sing_box_error_count",
		"unsupported_count",
	} {
		if !columnExists(t, store, "subscription_refresh_logs", column) {
			t.Fatalf("expected subscription_refresh_logs.%s to exist", column)
		}
	}
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "jnmproxy.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()
}

func TestMigrateUpgradesVersionOneSchema(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if _, err := store.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if _, err := store.ExecContext(ctx, migrations[0].sql); err != nil {
		t.Fatalf("apply legacy schema: %v", err)
	}
	if _, err := store.ExecContext(ctx, "INSERT INTO schema_migrations (version, name) VALUES (?, ?)", migrations[0].version, migrations[0].name); err != nil {
		t.Fatalf("record legacy migration: %v", err)
	}

	if err := Migrate(ctx, store); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if !columnExists(t, store, "proxy_nodes", "sing_box_status") {
		t.Fatal("expected legacy schema to be upgraded with sing_box_status")
	}
	if !tableExists(t, store, "sing_box_engine_states") {
		t.Fatal("expected legacy schema to be upgraded with sing_box_engine_states")
	}
	if !columnExists(t, store, "subscription_refresh_logs", "sing_box_supported_count") {
		t.Fatal("expected legacy schema to be upgraded with refresh protocol stats")
	}
	if !tableExists(t, store, "operation_logs") {
		t.Fatal("expected legacy schema to be upgraded with operation_logs")
	}
}

func tableExists(t *testing.T, store *sql.DB, table string) bool {
	t.Helper()

	var name string
	err := store.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	return name == table
}

func columnExists(t *testing.T, store *sql.DB, table string, column string) bool {
	t.Helper()

	rows, err := store.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("query table info: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info: %v", err)
	}
	return false
}
