package stats

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type Key struct {
	BucketAt       string
	CredentialID   int64
	SubscriptionID int64
	NodeID         int64
	GroupID        int64
}

type Counter struct {
	Connections        int64
	SuccessConnections int64
	FailedConnections  int64
	UploadBytes        int64
	DownloadBytes      int64
}

type Event struct {
	CredentialID   int64
	SubscriptionID int64
	NodeID         int64
	GroupID        int64
	UploadBytes    int64
	DownloadBytes  int64
	Success        bool
	At             time.Time
}

type Collector struct {
	mu     sync.Mutex
	now    func() time.Time
	hourly map[Key]Counter
	daily  map[Key]Counter
}

func NewCollector(now func() time.Time) *Collector {
	if now == nil {
		now = time.Now
	}
	return &Collector{
		now:    now,
		hourly: make(map[Key]Counter),
		daily:  make(map[Key]Counter),
	}
}

func (collector *Collector) Record(event Event) {
	at := event.At
	if at.IsZero() {
		at = collector.now()
	}
	at = at.UTC()
	hourKey := Key{
		BucketAt:       at.Truncate(time.Hour).Format(time.RFC3339),
		CredentialID:   event.CredentialID,
		SubscriptionID: event.SubscriptionID,
		NodeID:         event.NodeID,
		GroupID:        event.GroupID,
	}
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	dayKey := hourKey
	dayKey.BucketAt = day.Format(time.RFC3339)

	counter := Counter{
		Connections:   1,
		UploadBytes:   event.UploadBytes,
		DownloadBytes: event.DownloadBytes,
	}
	if event.Success {
		counter.SuccessConnections = 1
	} else {
		counter.FailedConnections = 1
	}

	collector.mu.Lock()
	defer collector.mu.Unlock()
	addCounter(collector.hourly, hourKey, counter)
	addCounter(collector.daily, dayKey, counter)
}

func (collector *Collector) Snapshot() (map[Key]Counter, map[Key]Counter) {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	return cloneCounters(collector.hourly), cloneCounters(collector.daily)
}

func (collector *Collector) Flush(ctx context.Context, db *sql.DB) error {
	collector.mu.Lock()
	hourly := collector.hourly
	daily := collector.daily
	collector.hourly = make(map[Key]Counter)
	collector.daily = make(map[Key]Counter)
	collector.mu.Unlock()

	if len(hourly) == 0 && len(daily) == 0 {
		return nil
	}
	if err := flushCounters(ctx, db, hourly, daily); err != nil {
		collector.mu.Lock()
		mergeCounters(collector.hourly, hourly)
		mergeCounters(collector.daily, daily)
		collector.mu.Unlock()
		return err
	}
	return nil
}

func flushCounters(ctx context.Context, db *sql.DB, hourly map[Key]Counter, daily map[Key]Counter) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin stats flush: %w", err)
	}
	defer tx.Rollback()

	if err := flushTable(ctx, tx, "traffic_stats_hourly", hourly); err != nil {
		return err
	}
	if err := flushTable(ctx, tx, "traffic_stats_daily", daily); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit stats flush: %w", err)
	}
	return nil
}

func flushTable(ctx context.Context, tx *sql.Tx, table string, counters map[Key]Counter) error {
	if len(counters) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
INSERT INTO %s
    (bucket_at, credential_id, subscription_id, node_id, group_id,
     connections, success_connections, failed_connections, upload_bytes, download_bytes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(bucket_at, credential_id, subscription_id, node_id, group_id) DO UPDATE SET
    connections = connections + excluded.connections,
    success_connections = success_connections + excluded.success_connections,
    failed_connections = failed_connections + excluded.failed_connections,
    upload_bytes = upload_bytes + excluded.upload_bytes,
    download_bytes = download_bytes + excluded.download_bytes,
    updated_at = datetime('now')
`, table))
	if err != nil {
		return fmt.Errorf("prepare stats flush %s: %w", table, err)
	}
	defer stmt.Close()

	for key, counter := range counters {
		if _, err := stmt.ExecContext(ctx, key.BucketAt, key.CredentialID, key.SubscriptionID,
			key.NodeID, key.GroupID, counter.Connections, counter.SuccessConnections,
			counter.FailedConnections, counter.UploadBytes, counter.DownloadBytes); err != nil {
			return fmt.Errorf("flush stats %s: %w", table, err)
		}
	}
	return nil
}

func addCounter(counters map[Key]Counter, key Key, counter Counter) {
	current := counters[key]
	current.Connections += counter.Connections
	current.SuccessConnections += counter.SuccessConnections
	current.FailedConnections += counter.FailedConnections
	current.UploadBytes += counter.UploadBytes
	current.DownloadBytes += counter.DownloadBytes
	counters[key] = current
}

func mergeCounters(dst map[Key]Counter, src map[Key]Counter) {
	for key, counter := range src {
		addCounter(dst, key, counter)
	}
}

func cloneCounters(src map[Key]Counter) map[Key]Counter {
	dst := make(map[Key]Counter, len(src))
	for key, counter := range src {
		dst[key] = counter
	}
	return dst
}
