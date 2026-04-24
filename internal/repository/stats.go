package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type StatsRepository struct {
	db *sql.DB
}

type StatsOverview struct {
	Connections        int64 `json:"connections"`
	SuccessConnections int64 `json:"success_connections"`
	FailedConnections  int64 `json:"failed_connections"`
	UploadBytes        int64 `json:"upload_bytes"`
	DownloadBytes      int64 `json:"download_bytes"`
}

func NewStatsRepository(db *sql.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

func (repo *StatsRepository) Overview(ctx context.Context) (*StatsOverview, error) {
	var overview StatsOverview
	if err := repo.db.QueryRowContext(ctx, `
SELECT
    COALESCE(SUM(connections), 0),
    COALESCE(SUM(success_connections), 0),
    COALESCE(SUM(failed_connections), 0),
    COALESCE(SUM(upload_bytes), 0),
    COALESCE(SUM(download_bytes), 0)
FROM traffic_stats_daily
`).Scan(&overview.Connections, &overview.SuccessConnections, &overview.FailedConnections,
		&overview.UploadBytes, &overview.DownloadBytes); err != nil {
		return nil, fmt.Errorf("query stats overview: %w", err)
	}
	return &overview, nil
}
