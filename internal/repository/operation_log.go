package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type OperationLogRepository struct {
	db *sql.DB
}

type CreateOperationLogParams struct {
	Actor      string
	Action     string
	TargetType string
	TargetID   int64
	Message    string
	DetailJSON string
	IP         string
	UserAgent  string
}

type OperationLogListFilter struct {
	Action     string
	TargetType string
	Search     string
	Page       int
	PageSize   int
}

type OperationLogListResult struct {
	Items    []model.OperationLog `json:"items"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

func NewOperationLogRepository(db *sql.DB) *OperationLogRepository {
	return &OperationLogRepository{db: db}
}

func (repo *OperationLogRepository) Create(ctx context.Context, params CreateOperationLogParams) error {
	if params.Action == "" {
		return nil
	}
	if params.DetailJSON == "" {
		params.DetailJSON = "{}"
	}
	if _, err := repo.db.ExecContext(ctx, `
INSERT INTO operation_logs (actor, action, target_type, target_id, message, detail_json, ip, user_agent)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, params.Actor, params.Action, params.TargetType, params.TargetID, params.Message, params.DetailJSON, params.IP, params.UserAgent); err != nil {
		return fmt.Errorf("create operation log: %w", err)
	}
	return nil
}

func (repo *OperationLogRepository) List(ctx context.Context, filter OperationLogListFilter) (*OperationLogListResult, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	filter.Page = page
	filter.PageSize = pageSize

	where, args := buildOperationLogWhere(filter)
	countQuery := "SELECT COUNT(*) FROM operation_logs " + where
	var total int
	if err := repo.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count operation logs: %w", err)
	}

	query := `
SELECT id, actor, action, target_type, target_id, message, detail_json, ip, user_agent, created_at
FROM operation_logs
` + where + "ORDER BY id DESC\nLIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list operation logs: %w", err)
	}
	defer rows.Close()

	logs := make([]model.OperationLog, 0)
	for rows.Next() {
		log, err := scanOperationLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operation logs: %w", err)
	}
	return &OperationLogListResult{Items: logs, Total: total, Page: page, PageSize: pageSize}, nil
}

func buildOperationLogWhere(filter OperationLogListFilter) (string, []any) {
	var conditions []string
	var args []any
	if filter.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.TargetType != "" {
		conditions = append(conditions, "target_type = ?")
		args = append(args, filter.TargetType)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + escapeLike(strings.ToLower(search)) + "%"
		conditions = append(conditions, `(LOWER(action) LIKE ? ESCAPE '\' OR LOWER(target_type) LIKE ? ESCAPE '\' OR LOWER(message) LIKE ? ESCAPE '\' OR LOWER(actor) LIKE ? ESCAPE '\' OR LOWER(ip) LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like, like)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND ") + "\n", args
}

func scanOperationLog(row scanner) (*model.OperationLog, error) {
	var log model.OperationLog
	if err := row.Scan(&log.ID, &log.Actor, &log.Action, &log.TargetType, &log.TargetID, &log.Message, &log.DetailJSON, &log.IP, &log.UserAgent, &log.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan operation log: %w", err)
	}
	return &log, nil
}
