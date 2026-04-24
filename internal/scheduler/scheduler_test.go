package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/repository"
	"github.com/jnmproxy/jnmproxy/internal/subscription"
)

func TestRunDueRefreshes(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "clash/1.18.0" {
			t.Fatalf("unexpected user agent: %s", r.UserAgent())
		}
		_, _ = w.Write([]byte(`
proxies:
  - name: "HTTP 节点"
    type: http
    server: 127.0.0.1
    port: 18080
`))
	}))
	defer server.Close()

	subRepo := repository.NewSubscriptionRepository(store)
	subscriptionItem, err := subRepo.Create(ctx, repository.CreateSubscriptionParams{
		Name:                   "due",
		URL:                    server.URL,
		Enabled:                true,
		RefreshIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	runtimeCache := cache.NewStore()
	manager := subscription.NewManager(subRepo, subscription.ManagerOptions{RequestTimeout: 2 * time.Second})
	scheduler := &Scheduler{
		DB:                  store,
		Cache:               runtimeCache,
		SubscriptionRepo:    subRepo,
		SubscriptionManager: manager,
		Now: func() time.Time {
			return time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
		},
	}

	count, err := scheduler.RunDueRefreshes(ctx)
	if err != nil {
		t.Fatalf("run due refreshes: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one refreshed subscription, got %d", count)
	}
	nodes, err := subRepo.ListNodesBySubscription(ctx, subscriptionItem.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected one node, got %d", len(nodes))
	}
	updated, err := subRepo.Get(ctx, subscriptionItem.ID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if updated.LastStatus != "success" || updated.NextRefreshAt == "" {
		t.Fatalf("unexpected subscription refresh fields: %#v", updated)
	}
}

func TestRunHealthChecks(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	subRepo := repository.NewSubscriptionRepository(store)
	subscriptionItem, err := subRepo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: "https://example.com", Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if err := subRepo.UpsertNodes(ctx, []repository.UpsertProxyNodeParams{
		{
			SubscriptionID:      subscriptionItem.ID,
			SubscriptionNodeKey: "node-1",
			Name:                "node 1",
			Protocol:            "http",
			Server:              "127.0.0.1",
			Port:                18080,
			RawConfigJSON:       "{}",
			AdapterStatus:       "supported",
		},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	healthRepo := repository.NewHealthRepository(store)
	scheduler := &Scheduler{
		HealthRepo:    healthRepo,
		HealthChecker: fakeChecker{status: "alive", latencyMS: 12},
		Now: func() time.Time {
			return time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
		},
	}

	count, err := scheduler.RunHealthChecks(ctx)
	if err != nil {
		t.Fatalf("run health checks: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one checked node, got %d", count)
	}
	nodes, err := subRepo.ListNodesBySubscription(ctx, subscriptionItem.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if nodes[0].AliveStatus != "alive" || nodes[0].LatencyMS == nil || *nodes[0].LatencyMS != 12 || nodes[0].FailCount != 0 {
		t.Fatalf("unexpected node health fields: %#v", nodes[0])
	}
}

type fakeChecker struct {
	status    string
	latencyMS int64
}

func (checker fakeChecker) Check(ctx context.Context, node model.ProxyNode) repository.NodeHealthResult {
	return repository.NodeHealthResult{
		Status:    checker.status,
		LatencyMS: &checker.latencyMS,
	}
}
