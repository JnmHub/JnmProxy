package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	if result.UnsupportedCount != 1 {
		t.Fatalf("expected one unsupported non-native node without sing-box builder, got %#v", result)
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
	if logs[0].UnsupportedCount != 1 {
		t.Fatalf("expected refresh log unsupported count, got %#v", logs[0])
	}
}

func TestManagerRefreshKeepsNodeStableWhenNameChanges(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	name := "香港 HK 01"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`
proxies:
  - name: "%s"
    type: ss
    server: hk.example.com
    port: 8388
    cipher: aes-128-gcm
    password: pass
`, name)))
	}))
	defer server.Close()

	repo := repository.NewSubscriptionRepository(store)
	subscription, err := repo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: server.URL, Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	manager := NewManager(repo, ManagerOptions{RequestTimeout: 2 * time.Second})

	if _, err := manager.Refresh(ctx, subscription.ID); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	nodes, err := repo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list first nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected one node after first refresh, got %d", len(nodes))
	}
	nodeID := nodes[0].ID

	name = "香港 HK 01 - 剩余流量 99%"
	result, err := manager.Refresh(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if result.NodeCount != 1 {
		t.Fatalf("expected one unique node in refresh result, got %#v", result)
	}
	nodes, err = repo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list second nodes: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != nodeID {
		t.Fatalf("expected refresh to update existing node %d, got %#v", nodeID, nodes)
	}
	if nodes[0].Name != name {
		t.Fatalf("expected node name to be updated, got %q", nodes[0].Name)
	}
	updated, err := repo.Get(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if updated.NodeCount != 1 {
		t.Fatalf("expected subscription node_count to stay 1, got %d", updated.NodeCount)
	}
}

func TestManagerRefreshAdoptsLegacyRawConfigKey(t *testing.T) {
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
		_, _ = w.Write([]byte(`
proxies:
  - name: "新名称"
    type: http
    server: same.example.com
    port: 8080
`))
	}))
	defer server.Close()

	repo := repository.NewSubscriptionRepository(store)
	subscription, err := repo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: server.URL, Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	rawConfigJSON := `{"name":"旧名称","port":8080,"server":"same.example.com","type":"http"}`
	legacyKey := legacyNodeKey(subscription.ID, "http", "same.example.com", 8080, "旧名称", rawConfigJSON)
	if err := repo.UpsertNodes(ctx, []repository.UpsertProxyNodeParams{{
		SubscriptionID:      subscription.ID,
		SubscriptionNodeKey: legacyKey,
		Name:                "旧名称",
		Protocol:            "http",
		Server:              "same.example.com",
		Port:                8080,
		RawConfigJSON:       rawConfigJSON,
		AdapterStatus:       "supported",
	}}); err != nil {
		t.Fatalf("insert legacy node: %v", err)
	}

	manager := NewManager(repo, ManagerOptions{RequestTimeout: 2 * time.Second})
	if _, err := manager.Refresh(ctx, subscription.ID); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	nodes, err := repo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected legacy node to be adopted instead of duplicated, got %#v", nodes)
	}
	if nodes[0].SubscriptionNodeKey != legacyKey || nodes[0].Name != "新名称" {
		t.Fatalf("expected legacy row to be updated, got %#v", nodes[0])
	}
}

func TestManagerRefreshRetriesUnsupportedClientPlaceholderWithDefaultUserAgent(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	seenUserAgents := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserAgents[r.UserAgent()]++
		if r.UserAgent() == "clash/1.18.0" {
			_, _ = w.Write([]byte(`
proxies:
  - name: "不支持您的代理软件"
    type: ss
    server: 127.0.0.1
    port: 6666
    cipher: aes-128-gcm
    password: placeholder
  - name: "请换用支持的代理软件"
    type: ss
    server: 127.0.0.1
    port: 6666
    cipher: aes-128-gcm
    password: placeholder
`))
			return
		}
		if r.UserAgent() != DefaultUserAgent {
			t.Fatalf("unexpected fallback user agent: %s", r.UserAgent())
		}
		_, _ = w.Write([]byte(`
proxies:
  - name: "真实节点 01"
    type: ss
    server: real-1.example.com
    port: 8388
    cipher: aes-128-gcm
    password: pass
  - name: "真实节点 02"
    type: vless
    server: real-2.example.com
    port: 443
    uuid: 00000000-0000-0000-0000-000000000000
`))
	}))
	defer server.Close()

	repo := repository.NewSubscriptionRepository(store)
	subscription, err := repo.Create(ctx, repository.CreateSubscriptionParams{
		Name:                   "fallback",
		URL:                    server.URL,
		UserAgent:              "clash/1.18.0",
		RefreshIntervalSeconds: 60,
		Enabled:                true,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	manager := NewManager(repo, ManagerOptions{
		RequestTimeout: 2 * time.Second,
		SingBoxBuilder: func(node ParsedNode) SingBoxBuildResult {
			return SingBoxBuildResult{Status: "supported", JSON: `{"type":"direct"}`}
		},
	})
	result, err := manager.Refresh(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("refresh subscription: %v", err)
	}
	if result.NodeCount != 2 {
		t.Fatalf("expected fallback real nodes, got %#v", result)
	}
	if seenUserAgents["clash/1.18.0"] != 1 || seenUserAgents[DefaultUserAgent] != 1 {
		t.Fatalf("expected old UA and default UA requests, got %#v", seenUserAgents)
	}
	nodes, err := repo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 2 || nodes[0].Server == "127.0.0.1" || nodes[1].Server == "127.0.0.1" {
		t.Fatalf("expected real nodes to be persisted, got %#v", nodes)
	}
}

func legacyNodeKey(subscriptionID int64, protocol string, server string, port int, name string, rawConfigJSON string) string {
	identity := fmt.Sprintf("%d|%s|%s|%d|%s|%s", subscriptionID, protocol, server, port, name, rawConfigJSON)
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func TestLooksLikeUnsupportedClientNodes(t *testing.T) {
	if !looksLikeUnsupportedClientNodes([]ParsedNode{
		{Name: "不支持您的代理软件", Server: "127.0.0.1", Port: 6666},
		{Name: "请换用支持的代理软件", Server: "localhost", Port: 6666},
	}) {
		t.Fatal("expected unsupported-client placeholder to be detected")
	}
	if looksLikeUnsupportedClientNodes([]ParsedNode{
		{Name: "本地测试节点", Server: "127.0.0.1", Port: 1080},
	}) {
		t.Fatal("ordinary local node should not be treated as unsupported-client placeholder")
	}
}

func TestManagerRefreshPersistsSingBoxFieldsAndStats(t *testing.T) {
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
    cipher: aes-128-gcm
    password: pass
  - name: "SSR 节点"
    type: ssr
    server: ssr.example.com
    port: 8388
`))
	}))
	defer server.Close()

	repo := repository.NewSubscriptionRepository(store)
	subscription, err := repo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: server.URL, Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	manager := NewManager(repo, ManagerOptions{
		RequestTimeout: 2 * time.Second,
		SingBoxBuilder: func(node ParsedNode) SingBoxBuildResult {
			switch node.Protocol {
			case "ss":
				return SingBoxBuildResult{
					JSON:          `{"type":"shadowsocks"}`,
					Status:        "supported",
					Version:       "v1.13.8",
					TransportType: "tcp",
					UDPSupported:  true,
				}
			case "ssr":
				return SingBoxBuildResult{Status: "error", Error: "unsupported sing-box protocol"}
			default:
				return SingBoxBuildResult{Status: "unsupported"}
			}
		},
	})
	result, err := manager.Refresh(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("refresh subscription: %v", err)
	}
	if result.SingBoxSupportedCount != 1 || result.SingBoxErrorCount != 1 || result.UnsupportedCount != 0 {
		t.Fatalf("unexpected sing-box refresh stats: %#v", result)
	}

	nodes, err := repo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if nodes[0].AdapterStatus != "supported" || nodes[0].SingBoxStatus != "unsupported" {
		t.Fatalf("unexpected http node status: %#v", nodes[0])
	}
	if nodes[1].AdapterStatus != "supported" || nodes[1].SingBoxStatus != "supported" || nodes[1].SingBoxOutboundJSON == "" {
		t.Fatalf("unexpected ss node sing-box fields: %#v", nodes[1])
	}
	if nodes[2].AdapterStatus != "error" || nodes[2].SingBoxStatus != "error" || nodes[2].SingBoxError == "" {
		t.Fatalf("unexpected ssr node sing-box fields: %#v", nodes[2])
	}

	logs, err := repo.ListRefreshLogs(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list refresh logs: %v", err)
	}
	if len(logs) != 1 || logs[0].SingBoxSupportedCount != 1 || logs[0].SingBoxErrorCount != 1 || logs[0].UnsupportedCount != 0 {
		t.Fatalf("unexpected refresh log stats: %#v", logs)
	}
}

func TestManagerRefreshInvalidatesChangedAndMissingSingBoxNodes(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	body := `
proxies:
  - name: "SS 节点"
    type: ss
    server: ss.example.com
    port: 8388
    cipher: aes-128-gcm
    password: pass
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	repo := repository.NewSubscriptionRepository(store)
	subscription, err := repo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: server.URL, Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	version := 1
	var invalidated []int64
	manager := NewManager(repo, ManagerOptions{
		RequestTimeout: 2 * time.Second,
		SingBoxBuilder: func(node ParsedNode) SingBoxBuildResult {
			if node.Protocol != "ss" {
				return SingBoxBuildResult{Status: "unsupported"}
			}
			return SingBoxBuildResult{
				JSON:    fmt.Sprintf(`{"type":"shadowsocks","version":%d}`, version),
				Status:  "supported",
				Version: "v1.13.8",
			}
		},
		NodeInvalidator: func(nodeID int64) {
			invalidated = append(invalidated, nodeID)
		},
	})

	if _, err := manager.Refresh(ctx, subscription.ID); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	nodes, err := repo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list first nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected one node, got %d", len(nodes))
	}
	nodeID := nodes[0].ID
	if len(invalidated) != 0 {
		t.Fatalf("new node should not invalidate adapters: %#v", invalidated)
	}

	version = 2
	if _, err := manager.Refresh(ctx, subscription.ID); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if len(invalidated) != 1 || invalidated[0] != nodeID {
		t.Fatalf("expected changed node invalidation for %d, got %#v", nodeID, invalidated)
	}

	invalidated = nil
	body = `
proxies:
  - name: "HTTP 节点"
    type: http
    server: http.example.com
    port: 8080
`
	if _, err := manager.Refresh(ctx, subscription.ID); err != nil {
		t.Fatalf("third refresh: %v", err)
	}
	if len(invalidated) != 1 || invalidated[0] != nodeID {
		t.Fatalf("expected missing node invalidation for %d, got %#v", nodeID, invalidated)
	}
}
