package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type ProxyRequestLogRepository struct {
	db *sql.DB
}

type CreateProxyRequestLogParams struct {
	EntryProtocol    string
	CredentialID     int64
	Username         string
	TargetAddress    string
	Status           string
	AttemptCount     int
	SelectedNodeID   int64
	SelectedNodeName string
	Error            string
	AttemptsJSON     string
	DurationMS       int64
}

type ProxyRequestLogListFilter struct {
	Search        string
	Status        string
	EntryProtocol string
	Page          int
	PageSize      int
}

type ProxyRequestLogListResult struct {
	Items    []model.ProxyRequestLog `json:"items"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

func NewProxyRequestLogRepository(db *sql.DB) *ProxyRequestLogRepository {
	return &ProxyRequestLogRepository{db: db}
}

func (repo *ProxyRequestLogRepository) Create(ctx context.Context, params CreateProxyRequestLogParams) error {
	if params.Status == "" {
		params.Status = "failed"
	}
	if params.AttemptsJSON == "" {
		params.AttemptsJSON = "[]"
	}
	if _, err := repo.db.ExecContext(ctx, `
INSERT INTO proxy_request_logs
    (entry_protocol, credential_id, username, target_address, status, attempt_count,
     selected_node_id, selected_node_name, error, attempts_json, duration_ms)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, params.EntryProtocol, params.CredentialID, params.Username, params.TargetAddress,
		params.Status, params.AttemptCount, params.SelectedNodeID, params.SelectedNodeName,
		params.Error, params.AttemptsJSON, params.DurationMS); err != nil {
		return fmt.Errorf("create proxy request log: %w", err)
	}
	return nil
}

func (repo *ProxyRequestLogRepository) List(ctx context.Context, filter ProxyRequestLogListFilter) (*ProxyRequestLogListResult, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	filter.Page = page
	filter.PageSize = pageSize

	where, args := buildProxyRequestLogWhere(filter)
	var total int
	if err := repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM proxy_request_logs "+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count proxy request logs: %w", err)
	}

	query := `
SELECT id, entry_protocol, credential_id, username, target_address, status, attempt_count,
       selected_node_id, selected_node_name, error, attempts_json, duration_ms, created_at
FROM proxy_request_logs
` + where + "ORDER BY id DESC\nLIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list proxy request logs: %w", err)
	}
	defer rows.Close()

	logs := make([]model.ProxyRequestLog, 0)
	for rows.Next() {
		log, err := scanProxyRequestLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proxy request logs: %w", err)
	}
	return &ProxyRequestLogListResult{Items: logs, Total: total, Page: page, PageSize: pageSize}, nil
}

func buildProxyRequestLogWhere(filter ProxyRequestLogListFilter) (string, []any) {
	var conditions []string
	var args []any
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.EntryProtocol != "" {
		conditions = append(conditions, "entry_protocol = ?")
		args = append(args, filter.EntryProtocol)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + escapeLike(strings.ToLower(search)) + "%"
		conditions = append(conditions, `(LOWER(username) LIKE ? ESCAPE '\' OR LOWER(target_address) LIKE ? ESCAPE '\' OR LOWER(selected_node_name) LIKE ? ESCAPE '\' OR LOWER(error) LIKE ? ESCAPE '\' OR LOWER(attempts_json) LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like, like)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND ") + "\n", args
}

func scanProxyRequestLog(row scanner) (*model.ProxyRequestLog, error) {
	var log model.ProxyRequestLog
	if err := row.Scan(&log.ID, &log.EntryProtocol, &log.CredentialID, &log.Username,
		&log.TargetAddress, &log.Status, &log.AttemptCount, &log.SelectedNodeID,
		&log.SelectedNodeName, &log.Error, &log.AttemptsJSON, &log.DurationMS,
		&log.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan proxy request log: %w", err)
	}
	return &log, nil
}
