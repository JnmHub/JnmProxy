package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type NodeRepository struct {
	db *sql.DB
}

type NodeListFilter struct {
	SubscriptionID int64
	GroupID        int64
	Protocol       string
	AliveStatus    string
	SingBoxStatus  string
	Enabled        *bool
}

func NewNodeRepository(db *sql.DB) *NodeRepository {
	return &NodeRepository{db: db}
}

func (repo *NodeRepository) Get(ctx context.Context, id int64) (*model.ProxyNode, error) {
	row := repo.db.QueryRowContext(ctx, `
SELECT id, subscription_id, subscription_node_key, name, protocol, server, port, raw_uri,
       raw_config_json, sing_box_outbound_json, sing_box_status, sing_box_error,
       sing_box_version, udp_supported, transport_type, adapter_status, enabled, alive_status, last_seen_at,
       last_checked_at, latency_ms, fail_count, created_at, updated_at
FROM proxy_nodes
WHERE id = ?
`, id)
	node, err := scanProxyNode(row)
	if err != nil {
		return nil, err
	}
	groupIDs, err := repo.listNodeGroupIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	node.GroupIDs = groupIDs
	return node, nil
}

func (repo *NodeRepository) List(ctx context.Context, filter NodeListFilter) ([]model.ProxyNode, error) {
	query := `
SELECT n.id, n.subscription_id, n.subscription_node_key, n.name, n.protocol, n.server, n.port,
       n.raw_uri, n.raw_config_json, n.sing_box_outbound_json, n.sing_box_status,
       n.sing_box_error, n.sing_box_version, n.udp_supported, n.transport_type,
       n.adapter_status, n.enabled, n.alive_status,
       n.last_seen_at, n.last_checked_at, n.latency_ms, n.fail_count, n.created_at, n.updated_at
FROM proxy_nodes n
`
	var conditions []string
	var args []any
	if filter.GroupID > 0 {
		query += "JOIN proxy_node_groups ng ON ng.node_id = n.id\n"
		conditions = append(conditions, "ng.group_id = ?")
		args = append(args, filter.GroupID)
	}
	if filter.SubscriptionID > 0 {
		conditions = append(conditions, "n.subscription_id = ?")
		args = append(args, filter.SubscriptionID)
	}
	if filter.Protocol != "" {
		conditions = append(conditions, "n.protocol = ?")
		args = append(args, filter.Protocol)
	}
	if filter.AliveStatus != "" {
		conditions = append(conditions, "n.alive_status = ?")
		args = append(args, filter.AliveStatus)
	}
	if filter.SingBoxStatus != "" {
		conditions = append(conditions, "n.sing_box_status = ?")
		args = append(args, filter.SingBoxStatus)
	}
	if filter.Enabled != nil {
		conditions = append(conditions, "n.enabled = ?")
		args = append(args, boolToInt(*filter.Enabled))
	}
	if len(conditions) > 0 {
		query += "WHERE " + strings.Join(conditions, " AND ") + "\n"
	}
	query += "ORDER BY n.id DESC"

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
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
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	if err := repo.attachNodeGroupIDs(ctx, nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (repo *NodeRepository) listNodeGroupIDs(ctx context.Context, nodeID int64) ([]int64, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT group_id
FROM proxy_node_groups
WHERE node_id = ?
ORDER BY group_id ASC
`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list node group ids: %w", err)
	}
	defer rows.Close()

	var groupIDs []int64
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			return nil, fmt.Errorf("scan node group id: %w", err)
		}
		groupIDs = append(groupIDs, groupID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node group ids: %w", err)
	}
	return groupIDs, nil
}

func (repo *NodeRepository) attachNodeGroupIDs(ctx context.Context, nodes []model.ProxyNode) error {
	if len(nodes) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(nodes)), ",")
	args := make([]any, 0, len(nodes))
	for _, node := range nodes {
		args = append(args, node.ID)
	}
	rows, err := repo.db.QueryContext(ctx, fmt.Sprintf(`
SELECT node_id, group_id
FROM proxy_node_groups
WHERE node_id IN (%s)
ORDER BY group_id ASC
`, placeholders), args...)
	if err != nil {
		return fmt.Errorf("attach node group ids: %w", err)
	}
	defer rows.Close()

	byNodeID := make(map[int64][]int64, len(nodes))
	for rows.Next() {
		var nodeID, groupID int64
		if err := rows.Scan(&nodeID, &groupID); err != nil {
			return fmt.Errorf("scan attached node group id: %w", err)
		}
		byNodeID[nodeID] = append(byNodeID[nodeID], groupID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate attached node group ids: %w", err)
	}
	for index := range nodes {
		nodes[index].GroupIDs = byNodeID[nodes[index].ID]
	}
	return nil
}

func (repo *NodeRepository) SetEnabled(ctx context.Context, ids []int64, enabled bool) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, boolToInt(enabled))
	for _, id := range ids {
		args = append(args, id)
	}

	query := fmt.Sprintf(`
UPDATE proxy_nodes
SET enabled = ?, updated_at = datetime('now')
WHERE id IN (%s)
`, placeholders)
	if _, err := repo.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("set nodes enabled: %w", err)
	}
	return nil
}
