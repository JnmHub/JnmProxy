package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/auth"
	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/grouping"
	"github.com/jnmproxy/jnmproxy/internal/repository"
	"github.com/jnmproxy/jnmproxy/internal/stats"
	"github.com/jnmproxy/jnmproxy/internal/subscription"
)

func TestSubscriptionCredentialAndGroupAPI(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	subscriptionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != subscription.DefaultUserAgent {
			t.Fatalf("unexpected user agent: %s", r.UserAgent())
		}
		w.Header().Set("subscription-userinfo", "upload=1; download=2; total=3; expire=1700000000")
		_, _ = w.Write([]byte(`
proxies:
  - name: "香港 HK 01"
    type: http
    server: 127.0.0.1
    port: 18080
`))
	}))
	defer subscriptionServer.Close()

	handler := newTestServer(t, store)
	subscriptionID := postJSONValue[int64](t, handler, "/api/v1/subscriptions", map[string]any{
		"name": "测试订阅",
		"url":  subscriptionServer.URL,
	}, "id")

	refresh := postJSON(t, handler, "/api/v1/subscriptions/"+itoa(subscriptionID)+"/refresh", map[string]any{})
	if refresh["node_count"].(float64) != 1 {
		t.Fatalf("unexpected refresh response: %#v", refresh)
	}

	nodesResponse := getJSON(t, handler, "/api/v1/nodes")
	nodes := nodesResponse.([]any)
	if len(nodes) != 1 {
		t.Fatalf("expected one node, got %d", len(nodes))
	}
	node := nodes[0].(map[string]any)
	nodeID := int64(node["id"].(float64))

	var rebuiltNodeID int64
	handler.NodeAdapterInvalidator = func(id int64) {
		rebuiltNodeID = id
	}
	singBoxStatus := getJSON(t, handler, "/api/v1/system/sing-box").(map[string]any)
	if singBoxStatus["enabled"] != true || singBoxStatus["adapter_configured"] != true {
		t.Fatalf("unexpected sing-box status: %#v", singBoxStatus)
	}
	rebuild := postJSON(t, handler, "/api/v1/nodes/"+itoa(nodeID)+"/rebuild-adapter", map[string]any{})
	if rebuild["status"] != "adapter_rebuild_scheduled" || rebuiltNodeID != nodeID {
		t.Fatalf("unexpected rebuild response=%#v rebuilt=%d", rebuild, rebuiltNodeID)
	}

	groupID := postJSONValue[int64](t, handler, "/api/v1/groups", map[string]any{"name": "手动分组"}, "id")
	request(t, handler, http.MethodPost, "/api/v1/groups/"+itoa(groupID)+"/nodes", map[string]any{"node_ids": []int64{nodeID}}, http.StatusNoContent)
	groupNodes := getJSON(t, handler, "/api/v1/nodes?group_id="+itoa(groupID)).([]any)
	if len(groupNodes) != 1 {
		t.Fatalf("expected one grouped node, got %d", len(groupNodes))
	}
	groupNode := groupNodes[0].(map[string]any)
	groupIDs := groupNode["group_ids"].([]any)
	if len(groupIDs) != 1 || int64(groupIDs[0].(float64)) != groupID {
		t.Fatalf("unexpected group ids on node response: %#v", groupNode)
	}
	statusFilteredNodes := getJSON(t, handler, "/api/v1/nodes?sing_box_status="+groupNode["sing_box_status"].(string)).([]any)
	if len(statusFilteredNodes) == 0 {
		t.Fatalf("expected sing-box status filter to return nodes")
	}

	credential := postJSON(t, handler, "/api/v1/credentials", map[string]any{
		"username":         "user",
		"password":         "pass",
		"bind_mode":        "group",
		"selection_policy": "fixed",
		"bindings": []map[string]any{
			{"target_type": "group", "target_id": groupID},
		},
	})
	if _, ok := credential["PasswordHash"]; ok {
		t.Fatalf("credential response leaked password hash: %#v", credential)
	}
	if _, ok := credential["password_hash"]; ok {
		t.Fatalf("credential response leaked password hash: %#v", credential)
	}
	if credential["selection_policy"] != "random" {
		t.Fatalf("expected group binding to normalize to random policy: %#v", credential)
	}
	bindings := credential["bindings"].([]any)
	if len(bindings) != 1 {
		t.Fatalf("expected one credential binding: %#v", credential)
	}
	binding := bindings[0].(map[string]any)
	if binding["target_type"] != "group" || int64(binding["target_id"].(float64)) != groupID {
		t.Fatalf("unexpected credential binding: %#v", credential)
	}

	overview := getJSON(t, handler, "/api/v1/stats/overview").(map[string]any)
	if overview["connections"].(float64) != 0 {
		t.Fatalf("unexpected stats overview: %#v", overview)
	}
}

func newTestServer(t *testing.T, store *sql.DB) *Server {
	t.Helper()
	runtimeCache := cache.NewStore()
	if err := runtimeCache.Load(context.Background(), store); err != nil {
		t.Fatalf("load cache: %v", err)
	}
	subRepo := repository.NewSubscriptionRepository(store)
	groupRepo := repository.NewGroupRepository(store)
	credentialRepo := repository.NewCredentialRepository(store)
	return &Server{
		DB:                  store,
		Cache:               runtimeCache,
		SubscriptionRepo:    subRepo,
		NodeRepo:            repository.NewNodeRepository(store),
		GroupRepo:           groupRepo,
		CredentialRepo:      credentialRepo,
		HealthRepo:          repository.NewHealthRepository(store),
		StatsRepo:           repository.NewStatsRepository(store),
		SubscriptionManager: subscription.NewManager(subRepo, subscription.ManagerOptions{RequestTimeout: 2 * time.Second}),
		AuthService:         auth.NewService(credentialRepo),
		GroupingService:     grouping.NewService(groupRepo),
		StatsCollector:      stats.NewCollector(time.Now),
		SingBoxStatus:       &SingBoxStatus{Enabled: true, Version: "v1.13.8", Mode: "auto"},
	}
}

func getJSON(t *testing.T, handler http.Handler, path string) any {
	t.Helper()
	return request(t, handler, http.MethodGet, path, nil, http.StatusOK)
}

func postJSON(t *testing.T, handler http.Handler, path string, body any) map[string]any {
	t.Helper()
	return request(t, handler, http.MethodPost, path, body, http.StatusOK).(map[string]any)
}

func postJSONValue[T ~int64](t *testing.T, handler http.Handler, path string, body any, key string) T {
	t.Helper()
	response := postJSON(t, handler, path, body)
	return T(int64(response[key].(float64)))
}

func request(t *testing.T, handler http.Handler, method string, path string, body any, expectedStatus int) any {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		content, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(content)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != expectedStatus {
		t.Fatalf("%s %s expected status %d got %d body=%s", method, path, expectedStatus, rec.Code, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		return nil
	}
	var response any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return response
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
