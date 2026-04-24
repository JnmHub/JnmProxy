package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type GroupRepository struct {
	db *sql.DB
}

func NewGroupRepository(db *sql.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

type CreateGroupParams struct {
	Name        string
	Description string
	AutoCreated bool
}

type UpdateGroupParams struct {
	Name        *string
	Description *string
	AutoCreated *bool
}

type CreateKeywordParams struct {
	Name          string
	Keywords      string
	CaseSensitive bool
	Enabled       bool
}

type UpdateKeywordParams struct {
	Name          *string
	Keywords      *string
	CaseSensitive *bool
	Enabled       *bool
}

func (repo *GroupRepository) CreateGroup(ctx context.Context, params CreateGroupParams) (*model.ProxyGroup, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, errors.New("group name is required")
	}
	result, err := repo.db.ExecContext(ctx, `
INSERT INTO proxy_groups (name, description, auto_created)
VALUES (?, ?, ?)
`, name, params.Description, boolToInt(params.AutoCreated))
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read group id: %w", err)
	}
	return repo.GetGroup(ctx, id)
}

func (repo *GroupRepository) EnsureGroup(ctx context.Context, name string, autoCreated bool) (*model.ProxyGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("group name is required")
	}
	if _, err := repo.db.ExecContext(ctx, `
INSERT OR IGNORE INTO proxy_groups (name, auto_created)
VALUES (?, ?)
`, name, boolToInt(autoCreated)); err != nil {
		return nil, fmt.Errorf("ensure group: %w", err)
	}
	return repo.GetGroupByName(ctx, name)
}

func (repo *GroupRepository) GetGroup(ctx context.Context, id int64) (*model.ProxyGroup, error) {
	row := repo.db.QueryRowContext(ctx, `
SELECT id, name, description, auto_created, created_at, updated_at
FROM proxy_groups
WHERE id = ?
`, id)
	return scanProxyGroup(row)
}

func (repo *GroupRepository) GetGroupByName(ctx context.Context, name string) (*model.ProxyGroup, error) {
	row := repo.db.QueryRowContext(ctx, `
SELECT id, name, description, auto_created, created_at, updated_at
FROM proxy_groups
WHERE name = ?
`, name)
	return scanProxyGroup(row)
}

func (repo *GroupRepository) ListGroups(ctx context.Context) ([]model.ProxyGroup, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, name, description, auto_created, created_at, updated_at
FROM proxy_groups
ORDER BY name ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []model.ProxyGroup
	for rows.Next() {
		group, err := scanProxyGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}
	return groups, nil
}

func (repo *GroupRepository) UpdateGroup(ctx context.Context, id int64, params UpdateGroupParams) (*model.ProxyGroup, error) {
	current, err := repo.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if params.Name != nil {
		current.Name = strings.TrimSpace(*params.Name)
	}
	if params.Description != nil {
		current.Description = *params.Description
	}
	if params.AutoCreated != nil {
		current.AutoCreated = *params.AutoCreated
	}
	if current.Name == "" {
		return nil, errors.New("group name is required")
	}

	if _, err := repo.db.ExecContext(ctx, `
UPDATE proxy_groups
SET name = ?, description = ?, auto_created = ?, updated_at = datetime('now')
WHERE id = ?
`, current.Name, current.Description, boolToInt(current.AutoCreated), id); err != nil {
		return nil, fmt.Errorf("update group: %w", err)
	}
	return repo.GetGroup(ctx, id)
}

func (repo *GroupRepository) DeleteGroup(ctx context.Context, id int64) error {
	if _, err := repo.db.ExecContext(ctx, "DELETE FROM proxy_groups WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}

func (repo *GroupRepository) AddNodesToGroup(ctx context.Context, groupID int64, nodeIDs []int64) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin add nodes to group: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO proxy_node_groups (node_id, group_id)
VALUES (?, ?)
`)
	if err != nil {
		return fmt.Errorf("prepare add nodes to group: %w", err)
	}
	defer stmt.Close()

	for _, nodeID := range nodeIDs {
		if _, err := stmt.ExecContext(ctx, nodeID, groupID); err != nil {
			return fmt.Errorf("add node %d to group %d: %w", nodeID, groupID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit add nodes to group: %w", err)
	}
	return nil
}

func (repo *GroupRepository) RemoveNodesFromGroup(ctx context.Context, groupID int64, nodeIDs []int64) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(nodeIDs)), ",")
	args := make([]any, 0, len(nodeIDs)+1)
	args = append(args, groupID)
	for _, nodeID := range nodeIDs {
		args = append(args, nodeID)
	}

	query := fmt.Sprintf("DELETE FROM proxy_node_groups WHERE group_id = ? AND node_id IN (%s)", placeholders)
	if _, err := repo.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("remove nodes from group: %w", err)
	}
	return nil
}

func (repo *GroupRepository) ListGroupsByNode(ctx context.Context, nodeID int64) ([]model.ProxyGroup, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT g.id, g.name, g.description, g.auto_created, g.created_at, g.updated_at
FROM proxy_groups g
JOIN proxy_node_groups ng ON ng.group_id = g.id
WHERE ng.node_id = ?
ORDER BY g.name ASC
`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list groups by node: %w", err)
	}
	defer rows.Close()

	var groups []model.ProxyGroup
	for rows.Next() {
		group, err := scanProxyGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node groups: %w", err)
	}
	return groups, nil
}

func (repo *GroupRepository) ListNodesByGroup(ctx context.Context, groupID int64) ([]model.ProxyNode, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT n.id, n.subscription_id, n.subscription_node_key, n.name, n.protocol, n.server, n.port,
       n.raw_uri, n.raw_config_json, n.adapter_status, n.enabled, n.alive_status,
       n.last_seen_at, n.last_checked_at, n.latency_ms, n.fail_count, n.created_at, n.updated_at
FROM proxy_nodes n
JOIN proxy_node_groups ng ON ng.node_id = n.id
WHERE ng.group_id = ?
ORDER BY n.id ASC
`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list nodes by group: %w", err)
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
		return nil, fmt.Errorf("iterate group nodes: %w", err)
	}
	return nodes, nil
}

func (repo *GroupRepository) ListNodeNames(ctx context.Context) ([]model.NodeName, error) {
	rows, err := repo.db.QueryContext(ctx, "SELECT id, name FROM proxy_nodes ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("list node names: %w", err)
	}
	defer rows.Close()

	var nodes []model.NodeName
	for rows.Next() {
		var node model.NodeName
		if err := rows.Scan(&node.ID, &node.Name); err != nil {
			return nil, fmt.Errorf("scan node name: %w", err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node names: %w", err)
	}
	return nodes, nil
}

func (repo *GroupRepository) CreateKeywordRule(ctx context.Context, params CreateKeywordParams) (*model.GroupKeyword, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, errors.New("keyword rule name is required")
	}
	if strings.TrimSpace(params.Keywords) == "" {
		return nil, errors.New("keywords are required")
	}
	result, err := repo.db.ExecContext(ctx, `
INSERT INTO group_keywords (name, keywords, case_sensitive, enabled)
VALUES (?, ?, ?, ?)
`, name, params.Keywords, boolToInt(params.CaseSensitive), boolToInt(params.Enabled))
	if err != nil {
		return nil, fmt.Errorf("create keyword rule: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read keyword rule id: %w", err)
	}
	return repo.GetKeywordRule(ctx, id)
}

func (repo *GroupRepository) GetKeywordRule(ctx context.Context, id int64) (*model.GroupKeyword, error) {
	row := repo.db.QueryRowContext(ctx, `
SELECT id, name, keywords, case_sensitive, enabled, created_at, updated_at
FROM group_keywords
WHERE id = ?
`, id)
	return scanGroupKeyword(row)
}

func (repo *GroupRepository) ListKeywordRules(ctx context.Context, onlyEnabled bool) ([]model.GroupKeyword, error) {
	query := `
SELECT id, name, keywords, case_sensitive, enabled, created_at, updated_at
FROM group_keywords
`
	if onlyEnabled {
		query += "WHERE enabled = 1\n"
	}
	query += "ORDER BY id ASC"

	rows, err := repo.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list keyword rules: %w", err)
	}
	defer rows.Close()

	var rules []model.GroupKeyword
	for rows.Next() {
		rule, err := scanGroupKeyword(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate keyword rules: %w", err)
	}
	return rules, nil
}

func (repo *GroupRepository) ListKeywordRulesByIDs(ctx context.Context, ids []int64) ([]model.GroupKeyword, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`
SELECT id, name, keywords, case_sensitive, enabled, created_at, updated_at
FROM group_keywords
WHERE enabled = 1 AND id IN (%s)
ORDER BY id ASC
`, placeholders)

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list keyword rules by ids: %w", err)
	}
	defer rows.Close()

	var rules []model.GroupKeyword
	for rows.Next() {
		rule, err := scanGroupKeyword(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate keyword rules by ids: %w", err)
	}
	return rules, nil
}

func (repo *GroupRepository) UpdateKeywordRule(ctx context.Context, id int64, params UpdateKeywordParams) (*model.GroupKeyword, error) {
	current, err := repo.GetKeywordRule(ctx, id)
	if err != nil {
		return nil, err
	}
	if params.Name != nil {
		current.Name = strings.TrimSpace(*params.Name)
	}
	if params.Keywords != nil {
		current.Keywords = *params.Keywords
	}
	if params.CaseSensitive != nil {
		current.CaseSensitive = *params.CaseSensitive
	}
	if params.Enabled != nil {
		current.Enabled = *params.Enabled
	}
	if current.Name == "" {
		return nil, errors.New("keyword rule name is required")
	}
	if strings.TrimSpace(current.Keywords) == "" {
		return nil, errors.New("keywords are required")
	}

	if _, err := repo.db.ExecContext(ctx, `
UPDATE group_keywords
SET name = ?, keywords = ?, case_sensitive = ?, enabled = ?, updated_at = datetime('now')
WHERE id = ?
`, current.Name, current.Keywords, boolToInt(current.CaseSensitive), boolToInt(current.Enabled), id); err != nil {
		return nil, fmt.Errorf("update keyword rule: %w", err)
	}
	return repo.GetKeywordRule(ctx, id)
}

func (repo *GroupRepository) DeleteKeywordRule(ctx context.Context, id int64) error {
	if _, err := repo.db.ExecContext(ctx, "DELETE FROM group_keywords WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete keyword rule: %w", err)
	}
	return nil
}

func scanProxyGroup(row scanner) (*model.ProxyGroup, error) {
	var group model.ProxyGroup
	var autoCreated int
	if err := row.Scan(&group.ID, &group.Name, &group.Description, &autoCreated, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan proxy group: %w", err)
	}
	group.AutoCreated = intToBool(autoCreated)
	return &group, nil
}

func scanGroupKeyword(row scanner) (*model.GroupKeyword, error) {
	var rule model.GroupKeyword
	var caseSensitive, enabled int
	if err := row.Scan(&rule.ID, &rule.Name, &rule.Keywords, &caseSensitive, &enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan group keyword: %w", err)
	}
	rule.CaseSensitive = intToBool(caseSensitive)
	rule.Enabled = intToBool(enabled)
	return &rule, nil
}
