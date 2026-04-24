package stats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/db"
)

func TestCollectorAggregatesAndFlushes(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	at := time.Date(2026, 4, 24, 10, 30, 0, 0, time.UTC)
	collector := NewCollector(func() time.Time { return at })
	collector.Record(Event{CredentialID: 1, SubscriptionID: 2, NodeID: 3, GroupID: 4, UploadBytes: 10, DownloadBytes: 20, Success: true})
	collector.Record(Event{CredentialID: 1, SubscriptionID: 2, NodeID: 3, GroupID: 4, UploadBytes: 5, DownloadBytes: 7, Success: false})

	hourly, daily := collector.Snapshot()
	if len(hourly) != 1 || len(daily) != 1 {
		t.Fatalf("unexpected snapshot sizes hourly=%d daily=%d", len(hourly), len(daily))
	}
	if err := collector.Flush(ctx, store); err != nil {
		t.Fatalf("flush stats: %v", err)
	}
	if err := collector.Flush(ctx, store); err != nil {
		t.Fatalf("second flush stats: %v", err)
	}

	var connections, success, failed, upload, download int64
	if err := store.QueryRowContext(ctx, `
SELECT connections, success_connections, failed_connections, upload_bytes, download_bytes
FROM traffic_stats_hourly
WHERE credential_id = 1 AND subscription_id = 2 AND node_id = 3 AND group_id = 4
`).Scan(&connections, &success, &failed, &upload, &download); err != nil {
		t.Fatalf("query hourly stats: %v", err)
	}
	if connections != 2 || success != 1 || failed != 1 || upload != 15 || download != 27 {
		t.Fatalf("unexpected hourly counters: connections=%d success=%d failed=%d upload=%d download=%d", connections, success, failed, upload, download)
	}

	if err := store.QueryRowContext(ctx, `
SELECT connections, success_connections, failed_connections, upload_bytes, download_bytes
FROM traffic_stats_daily
WHERE credential_id = 1 AND subscription_id = 2 AND node_id = 3 AND group_id = 4
`).Scan(&connections, &success, &failed, &upload, &download); err != nil {
		t.Fatalf("query daily stats: %v", err)
	}
	if connections != 2 || success != 1 || failed != 1 || upload != 15 || download != 27 {
		t.Fatalf("unexpected daily counters: connections=%d success=%d failed=%d upload=%d download=%d", connections, success, failed, upload, download)
	}
}
