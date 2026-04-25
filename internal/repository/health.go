package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type HealthRepository struct {
	db *sql.DB
}

func NewHealthRepository(db *sql.DB) *HealthRepository {
	return &HealthRepository{db: db}
}

type NodeHealthResult struct {
	NodeID    int64
	Status    string
	LatencyMS *int64
	Error     string
	CheckedAt string
}

func (repo *HealthRepository) ListCheckableNodes(ctx context.Context) ([]model.ProxyNode, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, subscription_id, subscription_node_key, name, protocol, server, port, raw_uri,
       raw_config_json, sing_box_outbound_json, sing_box_status, sing_box_error,
       sing_box_version, udp_supported, transport_type, adapter_status, enabled, alive_status, last_seen_at,
       last_checked_at, latency_ms, fail_count, created_at, updated_at
FROM proxy_nodes
WHERE enabled = 1 AND adapter_status = 'supported'
ORDER BY id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list checkable nodes: %w", err)
	}
	defer rows.Close()

	var nodes []model.ProxyNode
	for rows.Next() {
		node, err := scanProxyNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checkable nodes: %w", err)
	}
	return nodes, nil
}

func (repo *HealthRepository) RecordNodeHealth(ctx context.Context, result NodeHealthResult) error {
	status := result.Status
	if status != "alive" && status != "dead" {
		return fmt.Errorf("invalid health status %q", status)
	}

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin node health result: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO node_health_checks (node_id, status, latency_ms, error, checked_at)
VALUES (?, ?, ?, ?, ?)
`, result.NodeID, status, nullableInt64(result.LatencyMS), nullableString(result.Error), result.CheckedAt); err != nil {
		return fmt.Errorf("insert node health check: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE proxy_nodes
SET alive_status = ?,
    last_checked_at = ?,
    latency_ms = ?,
    fail_count = CASE WHEN ? = 'alive' THEN 0 ELSE fail_count + 1 END,
    updated_at = datetime('now')
WHERE id = ?
`, status, result.CheckedAt, nullableInt64(result.LatencyMS), status, result.NodeID); err != nil {
		return fmt.Errorf("update node health: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit node health result: %w", err)
	}
	return nil
}
