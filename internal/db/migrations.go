package db

import (
	"context"
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		name:    "initial_schema",
		sql: `
CREATE TABLE IF NOT EXISTS subscriptions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	user_agent TEXT NOT NULL DEFAULT 'clash/1.18.0',
	refresh_interval_seconds INTEGER NOT NULL DEFAULT 3600 CHECK (refresh_interval_seconds > 0),
	enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
	last_refresh_at TEXT,
	next_refresh_at TEXT,
	last_status TEXT NOT NULL DEFAULT 'never' CHECK (last_status IN ('success', 'failed', 'never')),
	last_error TEXT,
	upload_bytes INTEGER,
	download_bytes INTEGER,
	total_bytes INTEGER,
	expire_at TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS subscription_refresh_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	subscription_id INTEGER NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('success', 'failed')),
	http_status INTEGER,
	node_count INTEGER NOT NULL DEFAULT 0,
	error TEXT,
	started_at TEXT NOT NULL DEFAULT (datetime('now')),
	finished_at TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS proxy_nodes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	subscription_id INTEGER NOT NULL,
	subscription_node_key TEXT NOT NULL,
	name TEXT NOT NULL,
	protocol TEXT NOT NULL,
	server TEXT NOT NULL,
	port INTEGER NOT NULL CHECK (port > 0 AND port <= 65535),
	raw_uri TEXT,
	raw_config_json TEXT NOT NULL DEFAULT '{}',
	adapter_status TEXT NOT NULL DEFAULT 'unsupported' CHECK (adapter_status IN ('supported', 'unsupported', 'error')),
	enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
	alive_status TEXT NOT NULL DEFAULT 'unknown' CHECK (alive_status IN ('unknown', 'alive', 'dead')),
	last_seen_at TEXT,
	last_checked_at TEXT,
	latency_ms INTEGER,
	fail_count INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE CASCADE,
	UNIQUE (subscription_id, subscription_node_key)
);

CREATE TABLE IF NOT EXISTS proxy_groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	auto_created INTEGER NOT NULL DEFAULT 0 CHECK (auto_created IN (0, 1)),
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proxy_node_groups (
	node_id INTEGER NOT NULL,
	group_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	PRIMARY KEY (node_id, group_id),
	FOREIGN KEY (node_id) REFERENCES proxy_nodes(id) ON DELETE CASCADE,
	FOREIGN KEY (group_id) REFERENCES proxy_groups(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS group_keywords (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	keywords TEXT NOT NULL,
	case_sensitive INTEGER NOT NULL DEFAULT 0 CHECK (case_sensitive IN (0, 1)),
	enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS credentials (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
	bind_mode TEXT NOT NULL DEFAULT 'all' CHECK (bind_mode IN ('all', 'group', 'node')),
	selection_policy TEXT NOT NULL DEFAULT 'random' CHECK (selection_policy IN ('random', 'fixed')),
	remark TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS credential_bindings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	credential_id INTEGER NOT NULL,
	target_type TEXT NOT NULL CHECK (target_type IN ('group', 'node')),
	target_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (credential_id) REFERENCES credentials(id) ON DELETE CASCADE,
	UNIQUE (credential_id, target_type, target_id)
);

CREATE TABLE IF NOT EXISTS traffic_stats_hourly (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	bucket_at TEXT NOT NULL,
	credential_id INTEGER NOT NULL DEFAULT 0,
	subscription_id INTEGER NOT NULL DEFAULT 0,
	node_id INTEGER NOT NULL DEFAULT 0,
	group_id INTEGER NOT NULL DEFAULT 0,
	connections INTEGER NOT NULL DEFAULT 0,
	success_connections INTEGER NOT NULL DEFAULT 0,
	failed_connections INTEGER NOT NULL DEFAULT 0,
	upload_bytes INTEGER NOT NULL DEFAULT 0,
	download_bytes INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	UNIQUE (bucket_at, credential_id, subscription_id, node_id, group_id)
);

CREATE TABLE IF NOT EXISTS traffic_stats_daily (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	bucket_at TEXT NOT NULL,
	credential_id INTEGER NOT NULL DEFAULT 0,
	subscription_id INTEGER NOT NULL DEFAULT 0,
	node_id INTEGER NOT NULL DEFAULT 0,
	group_id INTEGER NOT NULL DEFAULT 0,
	connections INTEGER NOT NULL DEFAULT 0,
	success_connections INTEGER NOT NULL DEFAULT 0,
	failed_connections INTEGER NOT NULL DEFAULT 0,
	upload_bytes INTEGER NOT NULL DEFAULT 0,
	download_bytes INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	UNIQUE (bucket_at, credential_id, subscription_id, node_id, group_id)
);

CREATE TABLE IF NOT EXISTS node_health_checks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id INTEGER NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('alive', 'dead')),
	latency_ms INTEGER,
	error TEXT,
	checked_at TEXT NOT NULL DEFAULT (datetime('now')),
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (node_id) REFERENCES proxy_nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS system_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_next_refresh ON subscriptions(enabled, next_refresh_at);
CREATE INDEX IF NOT EXISTS idx_refresh_logs_subscription ON subscription_refresh_logs(subscription_id, created_at);
CREATE INDEX IF NOT EXISTS idx_nodes_subscription ON proxy_nodes(subscription_id, enabled);
CREATE INDEX IF NOT EXISTS idx_nodes_protocol ON proxy_nodes(protocol);
CREATE INDEX IF NOT EXISTS idx_nodes_health ON proxy_nodes(enabled, adapter_status, alive_status);
CREATE INDEX IF NOT EXISTS idx_group_keywords_enabled ON group_keywords(enabled);
CREATE INDEX IF NOT EXISTS idx_credentials_enabled ON credentials(enabled);
CREATE INDEX IF NOT EXISTS idx_traffic_hourly_bucket ON traffic_stats_hourly(bucket_at);
CREATE INDEX IF NOT EXISTS idx_traffic_daily_bucket ON traffic_stats_daily(bucket_at);
CREATE INDEX IF NOT EXISTS idx_health_node_checked ON node_health_checks(node_id, checked_at);
`,
	},
}

func Migrate(ctx context.Context, store *sql.DB) error {
	if store == nil {
		return fmt.Errorf("database handle is nil")
	}
	if _, err := store.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, item := range migrations {
		applied, err := migrationApplied(ctx, store, item.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, store, item); err != nil {
			return err
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, store *sql.DB, version int) (bool, error) {
	var exists int
	err := store.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ?", version).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("check migration %d: %w", version, err)
}

func applyMigration(ctx context.Context, store *sql.DB, item migration) error {
	tx, err := store.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", item.version, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, item.sql); err != nil {
		return fmt.Errorf("apply migration %d %s: %w", item.version, item.name, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, name) VALUES (?, ?)", item.version, item.name); err != nil {
		return fmt.Errorf("record migration %d %s: %w", item.version, item.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d %s: %w", item.version, item.name, err)
	}
	return nil
}
