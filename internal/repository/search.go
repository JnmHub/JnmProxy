package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type SearchRepository struct {
	db *sql.DB
}

func NewSearchRepository(db *sql.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

func (repo *SearchRepository) Search(ctx context.Context, query string) (*model.SearchResult, error) {
	query = strings.TrimSpace(query)
	result := &model.SearchResult{Query: query, Items: []model.SearchItem{}}
	if query == "" {
		return result, nil
	}
	like := "%" + escapeLike(strings.ToLower(query)) + "%"

	searchers := []func(context.Context, string, string) ([]model.SearchItem, error){
		repo.searchNodes,
		repo.searchSubscriptions,
		repo.searchGroups,
		repo.searchCredentials,
		repo.searchOperationLogs,
	}
	for _, searcher := range searchers {
		items, err := searcher(ctx, query, like)
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, items...)
	}
	return result, nil
}

func (repo *SearchRepository) searchNodes(ctx context.Context, query string, like string) ([]model.SearchItem, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, name, protocol, server, port
FROM proxy_nodes
WHERE LOWER(name) LIKE ? ESCAPE '\' OR LOWER(server) LIKE ? ESCAPE '\' OR LOWER(protocol) LIKE ? ESCAPE '\'
ORDER BY id DESC
LIMIT 5
`, like, like, like)
	if err != nil {
		return nil, fmt.Errorf("search nodes: %w", err)
	}
	defer rows.Close()

	var items []model.SearchItem
	for rows.Next() {
		var id int64
		var name, protocol, server string
		var port int
		if err := rows.Scan(&id, &name, &protocol, &server, &port); err != nil {
			return nil, fmt.Errorf("scan node search item: %w", err)
		}
		items = append(items, model.SearchItem{
			Type:     "node",
			ID:       id,
			Title:    name,
			Subtitle: fmt.Sprintf("节点 / %s / %s:%d", protocol, server, port),
			URL:      "/nodes?search=" + url.QueryEscape(query),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node search: %w", err)
	}
	return items, nil
}

func (repo *SearchRepository) searchSubscriptions(ctx context.Context, query string, like string) ([]model.SearchItem, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, name, url, last_status
FROM subscriptions
WHERE LOWER(name) LIKE ? ESCAPE '\' OR LOWER(url) LIKE ? ESCAPE '\'
ORDER BY id DESC
LIMIT 5
`, like, like)
	if err != nil {
		return nil, fmt.Errorf("search subscriptions: %w", err)
	}
	defer rows.Close()

	var items []model.SearchItem
	for rows.Next() {
		var id int64
		var name, link, status string
		if err := rows.Scan(&id, &name, &link, &status); err != nil {
			return nil, fmt.Errorf("scan subscription search item: %w", err)
		}
		items = append(items, model.SearchItem{
			Type:     "subscription",
			ID:       id,
			Title:    name,
			Subtitle: fmt.Sprintf("订阅 / %s / %s", status, link),
			URL:      fmt.Sprintf("/subscriptions/%d", id),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription search: %w", err)
	}
	return items, nil
}

func (repo *SearchRepository) searchGroups(ctx context.Context, query string, like string) ([]model.SearchItem, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, name, description
FROM proxy_groups
WHERE LOWER(name) LIKE ? ESCAPE '\' OR LOWER(description) LIKE ? ESCAPE '\'
ORDER BY name ASC
LIMIT 5
`, like, like)
	if err != nil {
		return nil, fmt.Errorf("search groups: %w", err)
	}
	defer rows.Close()

	var items []model.SearchItem
	for rows.Next() {
		var id int64
		var name, description string
		if err := rows.Scan(&id, &name, &description); err != nil {
			return nil, fmt.Errorf("scan group search item: %w", err)
		}
		if description == "" {
			description = "分组管理"
		}
		items = append(items, model.SearchItem{
			Type:     "group",
			ID:       id,
			Title:    name,
			Subtitle: "分组 / " + description,
			URL:      "/groups?search=" + url.QueryEscape(query),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group search: %w", err)
	}
	return items, nil
}

func (repo *SearchRepository) searchCredentials(ctx context.Context, query string, like string) ([]model.SearchItem, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, username, bind_mode, enabled
FROM credentials
WHERE LOWER(username) LIKE ? ESCAPE '\' OR LOWER(remark) LIKE ? ESCAPE '\'
ORDER BY id DESC
LIMIT 5
`, like, like)
	if err != nil {
		return nil, fmt.Errorf("search credentials: %w", err)
	}
	defer rows.Close()

	var items []model.SearchItem
	for rows.Next() {
		var id int64
		var username, bindMode string
		var enabled int
		if err := rows.Scan(&id, &username, &bindMode, &enabled); err != nil {
			return nil, fmt.Errorf("scan credential search item: %w", err)
		}
		status := "禁用"
		if enabled != 0 {
			status = "启用"
		}
		items = append(items, model.SearchItem{
			Type:     "credential",
			ID:       id,
			Title:    username,
			Subtitle: fmt.Sprintf("凭证 / %s / %s", bindMode, status),
			URL:      "/credentials?search=" + url.QueryEscape(query),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credential search: %w", err)
	}
	return items, nil
}

func (repo *SearchRepository) searchOperationLogs(ctx context.Context, query string, like string) ([]model.SearchItem, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, action, target_type, message
FROM operation_logs
WHERE LOWER(action) LIKE ? ESCAPE '\' OR LOWER(target_type) LIKE ? ESCAPE '\' OR LOWER(message) LIKE ? ESCAPE '\'
ORDER BY id DESC
LIMIT 5
`, like, like, like)
	if err != nil {
		return nil, fmt.Errorf("search operation logs: %w", err)
	}
	defer rows.Close()

	var items []model.SearchItem
	for rows.Next() {
		var id int64
		var action, targetType, message string
		if err := rows.Scan(&id, &action, &targetType, &message); err != nil {
			return nil, fmt.Errorf("scan operation log search item: %w", err)
		}
		if message == "" {
			message = action
		}
		items = append(items, model.SearchItem{
			Type:     "operation_log",
			ID:       id,
			Title:    message,
			Subtitle: fmt.Sprintf("操作日志 / %s / %s", targetType, action),
			URL:      "/operation-logs?search=" + url.QueryEscape(query),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operation log search: %w", err)
	}
	return items, nil
}
