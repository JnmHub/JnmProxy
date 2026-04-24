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
	} {
		if !tableExists(t, store, table) {
			t.Fatalf("expected table %s to exist", table)
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
