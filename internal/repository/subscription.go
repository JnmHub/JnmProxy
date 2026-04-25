package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type SubscriptionRepository struct {
	db *sql.DB
}

func NewSubscriptionRepository(db *sql.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

type CreateSubscriptionParams struct {
	Name                   string
	URL                    string
	UserAgent              string
	RefreshIntervalSeconds int
	Enabled                bool
}

type UpdateSubscriptionParams struct {
	Name                   *string
	URL                    *string
	UserAgent              *string
	RefreshIntervalSeconds *int
	Enabled                *bool
}

type SubscriptionRefreshResult struct {
	Status                string
	HTTPStatus            *int64
	NodeCount             int
	SingBoxSupportedCount int
	SingBoxErrorCount     int
	UnsupportedCount      int
	Error                 string
	StartedAt             string
	FinishedAt            string
	LastRefreshAt         string
	NextRefreshAt         string
	UploadBytes           *int64
	DownloadBytes         *int64
	TotalBytes            *int64
	ExpireAt              string
}

type UpsertProxyNodeParams struct {
	SubscriptionID      int64
	SubscriptionNodeKey string
	Name                string
	Protocol            string
	Server              string
	Port                int
	RawURI              string
	RawConfigJSON       string
	SingBoxOutboundJSON string
	SingBoxStatus       string
	SingBoxError        string
	SingBoxVersion      string
	UDPSupported        bool
	TransportType       string
	AdapterStatus       string
	LastSeenAt          string
}

func (repo *SubscriptionRepository) Create(ctx context.Context, params CreateSubscriptionParams) (*model.Subscription, error) {
	userAgent := params.UserAgent
	if userAgent == "" {
		userAgent = "clash/1.18.0"
	}
	refreshInterval := params.RefreshIntervalSeconds
	if refreshInterval <= 0 {
		refreshInterval = 3600
	}

	result, err := repo.db.ExecContext(ctx, `
INSERT INTO subscriptions (name, url, user_agent, refresh_interval_seconds, enabled)
VALUES (?, ?, ?, ?, ?)
`, params.Name, params.URL, userAgent, refreshInterval, boolToInt(params.Enabled))
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read subscription id: %w", err)
	}
	return repo.Get(ctx, id)
}

func (repo *SubscriptionRepository) Get(ctx context.Context, id int64) (*model.Subscription, error) {
	row := repo.db.QueryRowContext(ctx, `
SELECT id, name, url, user_agent, refresh_interval_seconds, enabled, last_refresh_at,
       next_refresh_at, last_status, last_error, upload_bytes, download_bytes,
       total_bytes, expire_at, created_at, updated_at
FROM subscriptions
WHERE id = ?
`, id)
	return scanSubscription(row)
}

func (repo *SubscriptionRepository) List(ctx context.Context) ([]model.Subscription, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, name, url, user_agent, refresh_interval_seconds, enabled, last_refresh_at,
       next_refresh_at, last_status, last_error, upload_bytes, download_bytes,
       total_bytes, expire_at, created_at, updated_at
FROM subscriptions
ORDER BY id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []model.Subscription
	for rows.Next() {
		subscription, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, *subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}
	return subscriptions, nil
}

func (repo *SubscriptionRepository) ListDue(ctx context.Context, now string) ([]model.Subscription, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, name, url, user_agent, refresh_interval_seconds, enabled, last_refresh_at,
       next_refresh_at, last_status, last_error, upload_bytes, download_bytes,
       total_bytes, expire_at, created_at, updated_at
FROM subscriptions
WHERE enabled = 1 AND (next_refresh_at IS NULL OR next_refresh_at = '' OR next_refresh_at <= ?)
ORDER BY id ASC
`, now)
	if err != nil {
		return nil, fmt.Errorf("list due subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []model.Subscription
	for rows.Next() {
		subscription, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, *subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due subscriptions: %w", err)
	}
	return subscriptions, nil
}

func (repo *SubscriptionRepository) Update(ctx context.Context, id int64, params UpdateSubscriptionParams) (*model.Subscription, error) {
	current, err := repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if params.Name != nil {
		current.Name = *params.Name
	}
	if params.URL != nil {
		current.URL = *params.URL
	}
	if params.UserAgent != nil {
		current.UserAgent = *params.UserAgent
	}
	if params.RefreshIntervalSeconds != nil {
		current.RefreshIntervalSeconds = *params.RefreshIntervalSeconds
	}
	if params.Enabled != nil {
		current.Enabled = *params.Enabled
	}

	if _, err := repo.db.ExecContext(ctx, `
UPDATE subscriptions
SET name = ?, url = ?, user_agent = ?, refresh_interval_seconds = ?,
    enabled = ?, updated_at = datetime('now')
WHERE id = ?
`, current.Name, current.URL, current.UserAgent, current.RefreshIntervalSeconds, boolToInt(current.Enabled), id); err != nil {
		return nil, fmt.Errorf("update subscription: %w", err)
	}
	return repo.Get(ctx, id)
}

func (repo *SubscriptionRepository) Delete(ctx context.Context, id int64) error {
	if _, err := repo.db.ExecContext(ctx, "DELETE FROM subscriptions WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}

func (repo *SubscriptionRepository) RecordRefreshResult(ctx context.Context, subscriptionID int64, result SubscriptionRefreshResult) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin refresh result: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE subscriptions
SET last_refresh_at = ?, next_refresh_at = ?, last_status = ?, last_error = ?,
    upload_bytes = ?, download_bytes = ?, total_bytes = ?, expire_at = ?,
    updated_at = datetime('now')
WHERE id = ?
`, nullableString(result.LastRefreshAt), nullableString(result.NextRefreshAt), result.Status,
		nullableString(result.Error), nullableInt64(result.UploadBytes), nullableInt64(result.DownloadBytes),
		nullableInt64(result.TotalBytes), nullableString(result.ExpireAt), subscriptionID); err != nil {
		return fmt.Errorf("update subscription refresh result: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO subscription_refresh_logs
    (subscription_id, status, http_status, node_count, sing_box_supported_count,
     sing_box_error_count, unsupported_count, error, started_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, subscriptionID, result.Status, nullableInt64(result.HTTPStatus), result.NodeCount,
		result.SingBoxSupportedCount, result.SingBoxErrorCount, result.UnsupportedCount,
		nullableString(result.Error), result.StartedAt, nullableString(result.FinishedAt)); err != nil {
		return fmt.Errorf("insert subscription refresh log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit refresh result: %w", err)
	}
	return nil
}

func (repo *SubscriptionRepository) UpsertNodes(ctx context.Context, nodes []UpsertProxyNodeParams) error {
	if len(nodes) == 0 {
		return nil
	}

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert nodes: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO proxy_nodes
    (subscription_id, subscription_node_key, name, protocol, server, port, raw_uri,
     raw_config_json, sing_box_outbound_json, sing_box_status, sing_box_error,
     sing_box_version, udp_supported, transport_type, adapter_status, enabled,
     alive_status, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 'unknown', ?)
ON CONFLICT(subscription_id, subscription_node_key) DO UPDATE SET
    name = excluded.name,
    protocol = excluded.protocol,
    server = excluded.server,
    port = excluded.port,
    raw_uri = excluded.raw_uri,
    raw_config_json = excluded.raw_config_json,
    sing_box_outbound_json = excluded.sing_box_outbound_json,
    sing_box_status = excluded.sing_box_status,
    sing_box_error = excluded.sing_box_error,
    sing_box_version = excluded.sing_box_version,
    udp_supported = excluded.udp_supported,
    transport_type = excluded.transport_type,
    adapter_status = excluded.adapter_status,
    enabled = 1,
    alive_status = CASE WHEN proxy_nodes.alive_status = 'dead' THEN 'unknown' ELSE proxy_nodes.alive_status END,
    last_seen_at = excluded.last_seen_at,
    updated_at = datetime('now')
`)
	if err != nil {
		return fmt.Errorf("prepare upsert nodes: %w", err)
	}
	defer stmt.Close()

	for _, node := range nodes {
		singBoxStatus := node.SingBoxStatus
		if singBoxStatus == "" {
			singBoxStatus = "unsupported"
		}
		adapterStatus := node.AdapterStatus
		if adapterStatus == "" {
			adapterStatus = "unsupported"
		}
		if _, err := stmt.ExecContext(ctx, node.SubscriptionID, node.SubscriptionNodeKey, node.Name,
			node.Protocol, node.Server, node.Port, nullableString(node.RawURI), node.RawConfigJSON,
			node.SingBoxOutboundJSON, singBoxStatus, node.SingBoxError,
			node.SingBoxVersion, boolToInt(node.UDPSupported), node.TransportType,
			adapterStatus, nullableString(node.LastSeenAt)); err != nil {
			return fmt.Errorf("upsert node %q: %w", node.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upsert nodes: %w", err)
	}
	return nil
}

func (repo *SubscriptionRepository) MarkMissingNodes(ctx context.Context, subscriptionID int64, seenKeys []string) error {
	if len(seenKeys) == 0 {
		if _, err := repo.db.ExecContext(ctx, `
UPDATE proxy_nodes
SET enabled = 0, alive_status = 'dead', updated_at = datetime('now')
WHERE subscription_id = ?
`, subscriptionID); err != nil {
			return fmt.Errorf("mark all nodes missing: %w", err)
		}
		return nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(seenKeys)), ",")
	args := make([]any, 0, len(seenKeys)+1)
	args = append(args, subscriptionID)
	for _, key := range seenKeys {
		args = append(args, key)
	}

	query := fmt.Sprintf(`
UPDATE proxy_nodes
SET enabled = 0, alive_status = 'dead', updated_at = datetime('now')
WHERE subscription_id = ? AND subscription_node_key NOT IN (%s)
`, placeholders)
	if _, err := repo.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mark missing nodes: %w", err)
	}
	return nil
}

func (repo *SubscriptionRepository) ListNodesBySubscription(ctx context.Context, subscriptionID int64) ([]model.ProxyNode, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, subscription_id, subscription_node_key, name, protocol, server, port, raw_uri,
       raw_config_json, sing_box_outbound_json, sing_box_status, sing_box_error,
       sing_box_version, udp_supported, transport_type, adapter_status, enabled, alive_status, last_seen_at,
       last_checked_at, latency_ms, fail_count, created_at, updated_at
FROM proxy_nodes
WHERE subscription_id = ?
ORDER BY id ASC
`, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("list subscription nodes: %w", err)
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
		return nil, fmt.Errorf("iterate proxy nodes: %w", err)
	}
	return nodes, nil
}

func (repo *SubscriptionRepository) ListRefreshLogs(ctx context.Context, subscriptionID int64) ([]model.SubscriptionRefreshLog, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, subscription_id, status, http_status, node_count, sing_box_supported_count,
       sing_box_error_count, unsupported_count, error, started_at, finished_at, created_at
FROM subscription_refresh_logs
WHERE subscription_id = ?
ORDER BY id DESC
`, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("list refresh logs: %w", err)
	}
	defer rows.Close()

	var logs []model.SubscriptionRefreshLog
	for rows.Next() {
		log, err := scanRefreshLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate refresh logs: %w", err)
	}
	return logs, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSubscription(row scanner) (*model.Subscription, error) {
	var subscription model.Subscription
	var enabled int
	var lastRefreshAt, nextRefreshAt, lastError, expireAt sql.NullString
	var uploadBytes, downloadBytes, totalBytes sql.NullInt64
	var lastStatus string

	if err := row.Scan(&subscription.ID, &subscription.Name, &subscription.URL, &subscription.UserAgent,
		&subscription.RefreshIntervalSeconds, &enabled, &lastRefreshAt, &nextRefreshAt, &lastStatus,
		&lastError, &uploadBytes, &downloadBytes, &totalBytes, &expireAt, &subscription.CreatedAt,
		&subscription.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan subscription: %w", err)
	}

	subscription.Enabled = intToBool(enabled)
	subscription.LastRefreshAt = nullStringValue(lastRefreshAt)
	subscription.NextRefreshAt = nullStringValue(nextRefreshAt)
	subscription.LastStatus = model.SubscriptionStatus(lastStatus)
	subscription.LastError = nullStringValue(lastError)
	subscription.UploadBytes = nullInt64Ptr(uploadBytes)
	subscription.DownloadBytes = nullInt64Ptr(downloadBytes)
	subscription.TotalBytes = nullInt64Ptr(totalBytes)
	subscription.ExpireAt = nullStringValue(expireAt)
	return &subscription, nil
}

func scanProxyNode(row scanner) (*model.ProxyNode, error) {
	var node model.ProxyNode
	var rawURI, singBoxOutboundJSON, singBoxError, singBoxVersion, transportType, lastSeenAt, lastCheckedAt sql.NullString
	var latencyMS sql.NullInt64
	var singBoxStatus, adapterStatus, aliveStatus string
	var udpSupported, enabled int

	if err := row.Scan(&node.ID, &node.SubscriptionID, &node.SubscriptionNodeKey, &node.Name,
		&node.Protocol, &node.Server, &node.Port, &rawURI, &node.RawConfigJSON, &singBoxOutboundJSON,
		&singBoxStatus, &singBoxError, &singBoxVersion, &udpSupported, &transportType, &adapterStatus,
		&enabled, &aliveStatus, &lastSeenAt, &lastCheckedAt, &latencyMS, &node.FailCount, &node.CreatedAt,
		&node.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan proxy node: %w", err)
	}

	node.RawURI = nullStringValue(rawURI)
	node.SingBoxOutboundJSON = nullStringValue(singBoxOutboundJSON)
	node.SingBoxStatus = model.SingBoxStatus(singBoxStatus)
	node.SingBoxError = nullStringValue(singBoxError)
	node.SingBoxVersion = nullStringValue(singBoxVersion)
	node.UDPSupported = intToBool(udpSupported)
	node.TransportType = nullStringValue(transportType)
	node.AdapterStatus = model.AdapterStatus(adapterStatus)
	node.Enabled = intToBool(enabled)
	node.AliveStatus = model.AliveStatus(aliveStatus)
	node.LastSeenAt = nullStringValue(lastSeenAt)
	node.LastCheckedAt = nullStringValue(lastCheckedAt)
	node.LatencyMS = nullInt64Ptr(latencyMS)
	return &node, nil
}

func scanRefreshLog(row scanner) (*model.SubscriptionRefreshLog, error) {
	var log model.SubscriptionRefreshLog
	var httpStatus sql.NullInt64
	var errorText, finishedAt sql.NullString

	if err := row.Scan(&log.ID, &log.SubscriptionID, &log.Status, &httpStatus, &log.NodeCount,
		&log.SingBoxSupportedCount, &log.SingBoxErrorCount, &log.UnsupportedCount, &errorText,
		&log.StartedAt, &finishedAt, &log.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan refresh log: %w", err)
	}

	log.HTTPStatus = nullInt64Ptr(httpStatus)
	log.Error = nullStringValue(errorText)
	log.FinishedAt = nullStringValue(finishedAt)
	return &log, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func intToBool(value int) bool {
	return value != 0
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
