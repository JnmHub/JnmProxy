package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/repository"
)

func TestManagerRefreshFetchesWithUserAgentAndPersistsNodes(t *testing.T) {
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
		if got := r.UserAgent(); got != "clash/1.18.0" {
			t.Fatalf("unexpected user agent: %s", got)
		}
		w.Header().Set("subscription-userinfo", "upload=100; download=200; total=1000; expire=1700000000")
		_, _ = w.Write([]byte(`
proxies:
  - name: "HTTP 节点"
    type: http
    server: http.example.com
    port: 8080
  - name: "SS 节点"
    type: ss
    server: ss.example.com
    port: 8388
`))
	}))
	defer server.Close()

	repo := repository.NewSubscriptionRepository(store)
	subscription, err := repo.Create(ctx, repository.CreateSubscriptionParams{
		Name:                   "测试订阅",
		URL:                    server.URL,
		UserAgent:              "clash/1.18.0",
		RefreshIntervalSeconds: 60,
		Enabled:                true,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	manager := NewManager(repo, ManagerOptions{RequestTimeout: 2 * time.Second})
	result, err := manager.Refresh(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("refresh subscription: %v", err)
	}
	if result.NodeCount != 2 || result.HTTPStatus != http.StatusOK {
		t.Fatalf("unexpected refresh result: %#v", result)
	}

	nodes, err := repo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].AdapterStatus != "supported" {
		t.Fatalf("expected http node supported, got %s", nodes[0].AdapterStatus)
	}
	if nodes[1].AdapterStatus != "unsupported" {
		t.Fatalf("expected ss node unsupported before outbound adapter, got %s", nodes[1].AdapterStatus)
	}

	updated, err := repo.Get(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("get updated subscription: %v", err)
	}
	if updated.UploadBytes == nil || *updated.UploadBytes != 100 {
		t.Fatalf("unexpected upload bytes: %#v", updated.UploadBytes)
	}
	if updated.ExpireAt != "2023-11-14T22:13:20Z" {
		t.Fatalf("unexpected expire time: %s", updated.ExpireAt)
	}
	if updated.LastStatus != "success" {
		t.Fatalf("unexpected last status: %s", updated.LastStatus)
	}

	logs, err := repo.ListRefreshLogs(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Status != "success" || logs[0].NodeCount != 2 {
		t.Fatalf("unexpected refresh logs: %#v", logs)
	}
}
