package singbox

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/model"
)

func TestAdapterDialContextWithEmbeddedOutbound(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("adapter-ok"))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}

	adapter := NewAdapter(AdapterOptions{
		MaxActiveEngines: 2,
		IdleTimeout:      time.Minute,
		DialTimeout:      5 * time.Second,
		LogLevel:         "disabled",
	})
	defer adapter.Close()

	conn, err := adapter.DialContext(context.Background(), cache.NodeSnapshot{
		ID:                  1,
		Protocol:            "vless",
		SingBoxStatus:       model.SingBoxStatusSupported,
		SingBoxOutboundJSON: `{"type":"direct","tag":"node-1"}`,
	}, targetURL.Host)
	if err != nil {
		t.Fatalf("dial through adapter: %v", err)
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", targetURL.Host); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "adapter-ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestAdapterClosesIdleEnginesAndRebuilds(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("idle-ok"))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}

	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	adapter := NewAdapter(AdapterOptions{
		MaxActiveEngines: 2,
		IdleTimeout:      time.Second,
		DialTimeout:      5 * time.Second,
		LogLevel:         "disabled",
	})
	adapter.now = func() time.Time { return now }
	defer adapter.Close()

	node := cache.NodeSnapshot{
		ID:                  1,
		Protocol:            "vless",
		SingBoxStatus:       model.SingBoxStatusSupported,
		SingBoxOutboundJSON: `{"type":"direct","tag":"node-1"}`,
	}
	conn, err := adapter.DialContext(context.Background(), node, targetURL.Host)
	if err != nil {
		t.Fatalf("first dial through adapter: %v", err)
	}
	_ = conn.Close()
	if count := adapterEntryCount(adapter); count != 1 {
		t.Fatalf("expected one cached engine, got %d", count)
	}

	now = now.Add(2 * time.Second)
	adapter.CloseIdle()
	if count := adapterEntryCount(adapter); count != 0 {
		t.Fatalf("expected idle engine to be closed, got %d", count)
	}

	conn, err = adapter.DialContext(context.Background(), node, targetURL.Host)
	if err != nil {
		t.Fatalf("dial after idle cleanup: %v", err)
	}
	_ = conn.Close()
	if count := adapterEntryCount(adapter); count != 1 {
		t.Fatalf("expected engine to be rebuilt, got %d", count)
	}
}

func adapterEntryCount(adapter *Adapter) int {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	return len(adapter.entries)
}
