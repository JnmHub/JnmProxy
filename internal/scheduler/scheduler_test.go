package scheduler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/outbound"
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

func TestRunHealthChecksIncludesSingBoxNodes(t *testing.T) {
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
			Name:                "vmess",
			Protocol:            "vmess",
			Server:              "127.0.0.1",
			Port:                10001,
			RawConfigJSON:       "{}",
			AdapterStatus:       "unsupported",
		},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if _, err := store.ExecContext(ctx, `
UPDATE proxy_nodes
SET sing_box_status = 'supported',
    sing_box_outbound_json = '{"type":"vmess"}',
    sing_box_version = 'v1.13.8'
WHERE subscription_id = ?
`, subscriptionItem.ID); err != nil {
		t.Fatalf("mark sing-box supported: %v", err)
	}

	nodes, err := repository.NewHealthRepository(store).ListCheckableNodes(ctx)
	if err != nil {
		t.Fatalf("list checkable nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected one sing-box checkable node, got %d", len(nodes))
	}
	if nodes[0].SingBoxStatus != model.SingBoxStatusSupported || nodes[0].AdapterStatus != model.AdapterStatusUnsupported {
		t.Fatalf("unexpected sing-box node fields: %#v", nodes[0])
	}
}

func TestOutboundHealthCheckerUsesSingBoxFields(t *testing.T) {
	fakeDialer := &fakeSingBoxDialer{supported: true}
	checker := NewOutboundHealthChecker(&outbound.Dialer{Timeout: time.Second, SingBox: fakeDialer}, "example.com:443", time.Second)

	result := checker.Check(context.Background(), model.ProxyNode{
		ID:                  10,
		SubscriptionID:      20,
		Name:                "vmess",
		Protocol:            "vmess",
		Server:              "127.0.0.1",
		Port:                10001,
		RawConfigJSON:       "{}",
		SingBoxOutboundJSON: `{"type":"vmess"}`,
		SingBoxStatus:       model.SingBoxStatusSupported,
		SingBoxVersion:      "v1.13.8",
		UDPSupported:        true,
		TransportType:       "ws",
	})
	if result.Status != "alive" {
		t.Fatalf("expected alive result, got %#v", result)
	}
	if fakeDialer.targetAddress != "example.com:443" {
		t.Fatalf("unexpected target address %q", fakeDialer.targetAddress)
	}
	if fakeDialer.node.SingBoxOutboundJSON == "" || fakeDialer.node.SingBoxStatus != model.SingBoxStatusSupported {
		t.Fatalf("sing-box fields were not passed to dialer: %#v", fakeDialer.node)
	}
}

func TestRunHealthChecksReloadsCacheForDeadAndRecoveredNodes(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	credentialRepo := repository.NewCredentialRepository(store)
	if _, err := credentialRepo.Create(ctx, repository.CreateCredentialParams{
		Username:        "health-user",
		PasswordHash:    "hash",
		Enabled:         true,
		BindMode:        "all",
		SelectionPolicy: "fixed",
	}); err != nil {
		t.Fatalf("create credential: %v", err)
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

	runtimeCache := cache.NewStore()
	if err := runtimeCache.Load(ctx, store); err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if _, err := runtimeCache.Select("health-user"); err != nil {
		t.Fatalf("expected node before health check: %v", err)
	}

	healthRepo := repository.NewHealthRepository(store)
	checker := &queuedChecker{results: []repository.NodeHealthResult{
		{Status: "dead", Error: "dial failed"},
		{Status: "alive", LatencyMS: int64Ptr(15)},
	}}
	scheduler := &Scheduler{
		DB:            store,
		Cache:         runtimeCache,
		HealthRepo:    healthRepo,
		HealthChecker: checker,
	}

	if _, err := scheduler.RunHealthChecks(ctx); err != nil {
		t.Fatalf("run dead health check: %v", err)
	}
	if _, err := runtimeCache.Select("health-user"); !errors.Is(err, cache.ErrNoCandidateNodes) {
		t.Fatalf("expected dead node to leave cache candidates, got %v", err)
	}

	if _, err := scheduler.RunHealthChecks(ctx); err != nil {
		t.Fatalf("run recovered health check: %v", err)
	}
	if _, err := runtimeCache.Select("health-user"); err != nil {
		t.Fatalf("expected recovered node in cache candidates: %v", err)
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

type queuedChecker struct {
	results []repository.NodeHealthResult
	index   int
}

func (checker *queuedChecker) Check(ctx context.Context, node model.ProxyNode) repository.NodeHealthResult {
	if checker.index >= len(checker.results) {
		return checker.results[len(checker.results)-1]
	}
	result := checker.results[checker.index]
	checker.index++
	return result
}

type fakeSingBoxDialer struct {
	supported     bool
	node          cache.NodeSnapshot
	targetAddress string
}

func (dialer *fakeSingBoxDialer) DialContext(ctx context.Context, node cache.NodeSnapshot, targetAddress string) (net.Conn, error) {
	dialer.node = node
	dialer.targetAddress = targetAddress
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

func (dialer *fakeSingBoxDialer) Supports(protocol string) bool {
	return dialer.supported
}

func (dialer *fakeSingBoxDialer) CloseNode(nodeID int64) error {
	return nil
}

func int64Ptr(value int64) *int64 {
	return &value
}
