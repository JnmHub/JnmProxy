package proxy

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/auth"
	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/outbound"
	"github.com/jnmproxy/jnmproxy/internal/repository"
	"github.com/jnmproxy/jnmproxy/internal/stats"
)

func TestHTTPProxyGETThroughHTTPOutbound(t *testing.T) {
	ctx := context.Background()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok:" + r.URL.Path))
	}))
	defer target.Close()

	upstream := newHTTPConnectProxy(t)
	defer upstream.Close()

	runtimeCache := testRuntimeCache(t, ctx, upstream.URL)
	statsCollector := stats.NewCollector(func() time.Time {
		return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	})
	handler := NewHTTPProxy(runtimeCache, outbound.NewDialer(2*time.Second))
	handler.Stats = statsCollector
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, target.URL+"/hello", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "ok:/hello" {
		t.Fatalf("unexpected response status=%d body=%q", resp.StatusCode, string(body))
	}

	hourly, _ := statsCollector.Snapshot()
	if len(hourly) != 1 {
		t.Fatalf("expected one hourly stats key, got %d", len(hourly))
	}
	for _, counter := range hourly {
		if counter.Connections != 1 || counter.SuccessConnections != 1 || counter.DownloadBytes != int64(len(body)) {
			t.Fatalf("unexpected stats counter: %#v", counter)
		}
	}
}

func TestHTTPProxyRetriesAnotherNodeWhenFirstDialFails(t *testing.T) {
	ctx := context.Background()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("retry-ok"))
	}))
	defer target.Close()

	badUpstreamURL := "http://" + closedLocalAddress(t)
	goodUpstream := newHTTPConnectProxy(t)
	defer goodUpstream.Close()

	runtimeCache := testRuntimeCacheWithHTTPUpstreams(t, ctx, []string{badUpstreamURL, goodUpstream.URL})
	statsCollector := stats.NewCollector(func() time.Time {
		return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	})
	handler := NewHTTPProxy(runtimeCache, outbound.NewDialer(500*time.Millisecond))
	handler.Stats = statsCollector
	handler.MaxAttemptsPerRequest = 2
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest(http.MethodGet, target.URL+"/retry", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("proxy request %d: %v", i, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("read body %d: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK || string(body) != "retry-ok" {
			t.Fatalf("unexpected retry response %d status=%d body=%q", i, resp.StatusCode, string(body))
		}
	}

	hourly, _ := statsCollector.Snapshot()
	var successes, failures int64
	for _, counter := range hourly {
		successes += counter.SuccessConnections
		failures += counter.FailedConnections
	}
	if successes != 2 {
		t.Fatalf("expected two successful final requests, got %d", successes)
	}
	if failures < 1 {
		t.Fatalf("expected at least one failed upstream attempt before retry, got %d", failures)
	}
}

func TestHTTPProxyWritesFailedRequestLogWhenAllAttemptsFail(t *testing.T) {
	ctx := context.Background()
	runtimeCache, storeDB := testRuntimeCacheWithHTTPUpstreamsAndDB(t, ctx, []string{
		"http://" + closedLocalAddress(t),
		"http://" + closedLocalAddress(t),
	}, true)
	logRepo := repository.NewProxyRequestLogRepository(storeDB)

	handler := NewHTTPProxy(runtimeCache, outbound.NewDialer(200*time.Millisecond))
	handler.MaxAttemptsPerRequest = 2
	handler.RequestLogger = &RequestLogger{Repo: logRepo, RecordFailedOnly: true}
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.com/fail", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("proxy request should return 502 response, got error: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502 after all attempts fail, got %d", resp.StatusCode)
	}

	logs, err := logRepo.List(ctx, repository.ProxyRequestLogListFilter{Search: "example.com", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list proxy request logs: %v", err)
	}
	if logs.Total != 1 || len(logs.Items) != 1 {
		t.Fatalf("expected one failed request log, got %#v", logs)
	}
	logItem := logs.Items[0]
	if logItem.Username != "user" || logItem.Status != "failed" || logItem.AttemptCount != 2 {
		t.Fatalf("unexpected failed request log: %#v", logItem)
	}
}

func TestHTTPProxyConcurrentRequests(t *testing.T) {
	ctx := context.Background()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	upstream := newHTTPConnectProxy(t)
	defer upstream.Close()

	runtimeCache := testRuntimeCache(t, ctx, upstream.URL)
	statsCollector := stats.NewCollector(func() time.Time {
		return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	})
	handler := NewHTTPProxy(runtimeCache, outbound.NewDialer(2*time.Second))
	handler.Stats = statsCollector
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}

	const requestCount = 12
	var wg sync.WaitGroup
	errCh := make(chan error, requestCount)
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodGet, target.URL+"/concurrent/"+strconv.Itoa(index), nil)
			if err != nil {
				errCh <- err
				return
			}
			req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")))
			resp, err := client.Do(req)
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				errCh <- err
				return
			}
			if resp.StatusCode != http.StatusOK || string(body) != "ok" {
				errCh <- fmt.Errorf("unexpected response status=%d body=%q", resp.StatusCode, string(body))
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	hourly, _ := statsCollector.Snapshot()
	var connections int64
	for _, counter := range hourly {
		connections += counter.Connections
	}
	if connections != requestCount {
		t.Fatalf("expected %d connections, got %d", requestCount, connections)
	}
}

func TestHTTPProxyRequestSurvivesCacheReload(t *testing.T) {
	ctx := context.Background()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		<-release
		_, _ = w.Write([]byte("reload-ok"))
	}))
	defer target.Close()

	upstream := newHTTPConnectProxy(t)
	defer upstream.Close()

	runtimeCache, storeDB := testRuntimeCacheWithDB(t, ctx, upstream.URL)
	handler := NewHTTPProxy(runtimeCache, outbound.NewDialer(2*time.Second))
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, target.URL+"/during-reload", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")))

	type responseResult struct {
		body string
		err  error
	}
	resultCh := make(chan responseResult, 1)
	go func() {
		resp, err := client.Do(req)
		if err != nil {
			resultCh <- responseResult{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resultCh <- responseResult{err: err}
			return
		}
		if resp.StatusCode != http.StatusOK {
			resultCh <- responseResult{err: fmt.Errorf("unexpected status %d", resp.StatusCode)}
			return
		}
		resultCh <- responseResult{body: string(body)}
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach target before cache reload")
	}
	if _, err := storeDB.ExecContext(ctx, "UPDATE proxy_nodes SET alive_status = 'dead'"); err != nil {
		t.Fatalf("mark node dead: %v", err)
	}
	if err := runtimeCache.Load(ctx, storeDB); err != nil {
		t.Fatalf("reload cache: %v", err)
	}
	close(release)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("request failed after cache reload: %v", result.err)
		}
		if result.body != "reload-ok" {
			t.Fatalf("unexpected body after cache reload: %q", result.body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not finish after cache reload")
	}
}

func TestSOCKS5ProxyThroughHTTPOutbound(t *testing.T) {
	ctx := context.Background()
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen echo: %v", err)
	}
	defer echoListener.Close()
	go func() {
		conn, err := echoListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	upstream := newHTTPConnectProxy(t)
	defer upstream.Close()
	runtimeCache := testRuntimeCache(t, ctx, upstream.URL)

	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen socks: %v", err)
	}
	defer socksListener.Close()
	go func() {
		_ = NewSOCKS5Server(runtimeCache, outbound.NewDialer(2*time.Second)).Serve(socksListener)
	}()

	conn, err := net.Dial("tcp", socksListener.Addr().String())
	if err != nil {
		t.Fatalf("dial socks: %v", err)
	}
	defer conn.Close()
	if err := socks5Connect(conn, "user", "pass", echoListener.Addr().String()); err != nil {
		t.Fatalf("socks connect: %v", err)
	}

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read ping: %v", err)
	}
	if string(reply) != "ping" {
		t.Fatalf("unexpected echo reply: %q", string(reply))
	}
}

func newHTTPConnectProxy(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "connect required", http.StatusMethodNotAllowed)
			return
		}
		targetConn, err := net.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, "dial target failed", http.StatusBadGateway)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			_ = targetConn.Close()
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			_ = targetConn.Close()
			return
		}
		if _, err := io.WriteString(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			_ = clientConn.Close()
			_ = targetConn.Close()
			return
		}
		pipeConnections(clientConn, targetConn)
	}))
}

func testRuntimeCache(t *testing.T, ctx context.Context, upstreamURL string) *cache.Store {
	t.Helper()
	runtimeCache, _ := testRuntimeCacheWithDB(t, ctx, upstreamURL)
	return runtimeCache
}

func testRuntimeCacheWithDB(t *testing.T, ctx context.Context, upstreamURL string) (*cache.Store, *sql.DB) {
	t.Helper()

	return testRuntimeCacheWithHTTPUpstreamsAndDB(t, ctx, []string{upstreamURL}, false)
}

func testRuntimeCacheWithHTTPUpstreams(t *testing.T, ctx context.Context, upstreamURLs []string) *cache.Store {
	t.Helper()
	runtimeCache, _ := testRuntimeCacheWithHTTPUpstreamsAndDB(t, ctx, upstreamURLs, true)
	return runtimeCache
}

func testRuntimeCacheWithHTTPUpstreamsAndDB(t *testing.T, ctx context.Context, upstreamURLs []string, bindGroup bool) (*cache.Store, *sql.DB) {
	t.Helper()

	storeDB, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = storeDB.Close() })
	if err := db.Migrate(ctx, storeDB); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	subRepo := repository.NewSubscriptionRepository(storeDB)
	subscription, err := subRepo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: "https://example.com/sub", Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	upserts := make([]repository.UpsertProxyNodeParams, 0, len(upstreamURLs))
	for index, upstreamURL := range upstreamURLs {
		upstreamParsed, err := url.Parse(upstreamURL)
		if err != nil {
			t.Fatalf("parse upstream url: %v", err)
		}
		host, portText, err := net.SplitHostPort(upstreamParsed.Host)
		if err != nil {
			t.Fatalf("split upstream host: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse upstream port: %v", err)
		}
		upserts = append(upserts, repository.UpsertProxyNodeParams{
			SubscriptionID:      subscription.ID,
			SubscriptionNodeKey: fmt.Sprintf("http-upstream-%d", index),
			Name:                fmt.Sprintf("HTTP upstream %d", index),
			Protocol:            "http",
			Server:              host,
			Port:                port,
			RawConfigJSON:       "{}",
			AdapterStatus:       "supported",
		})
	}
	if err := subRepo.UpsertNodes(ctx, upserts); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	nodes, err := subRepo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}

	authService := auth.NewService(repository.NewCredentialRepository(storeDB))
	input := auth.CreateCredentialInput{
		Username:        "user",
		Password:        "pass",
		Enabled:         true,
		BindMode:        "all",
		SelectionPolicy: "fixed",
	}
	if bindGroup {
		groupRepo := repository.NewGroupRepository(storeDB)
		group, err := groupRepo.CreateGroup(ctx, repository.CreateGroupParams{Name: "retry-group"})
		if err != nil {
			t.Fatalf("create retry group: %v", err)
		}
		nodeIDs := make([]int64, 0, len(nodes))
		for _, node := range nodes {
			nodeIDs = append(nodeIDs, node.ID)
		}
		if err := groupRepo.AddNodesToGroup(ctx, group.ID, nodeIDs); err != nil {
			t.Fatalf("add nodes to retry group: %v", err)
		}
		input.BindMode = "group"
		input.SelectionPolicy = "random"
		input.Bindings = []repository.CredentialBindingTarget{{TargetType: "group", TargetID: group.ID}}
	}
	if _, err := authService.CreateCredential(ctx, input); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	runtimeCache := cache.NewStore()
	if err := runtimeCache.Load(ctx, storeDB); err != nil {
		t.Fatalf("load cache: %v", err)
	}
	return runtimeCache, storeDB
}

func closedLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen closed address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	return address
}

func socks5Connect(conn net.Conn, username string, password string, target string) error {
	reader := bufio.NewReader(conn)
	if _, err := conn.Write([]byte{0x05, 0x01, 0x02}); err != nil {
		return err
	}
	method := []byte{0, 0}
	if _, err := io.ReadFull(reader, method); err != nil {
		return err
	}
	if method[1] != 0x02 {
		return fmt.Errorf("unexpected socks method %d", method[1])
	}

	authRequest := []byte{0x01, byte(len(username))}
	authRequest = append(authRequest, []byte(username)...)
	authRequest = append(authRequest, byte(len(password)))
	authRequest = append(authRequest, []byte(password)...)
	if _, err := conn.Write(authRequest); err != nil {
		return err
	}
	authResponse := []byte{0, 0}
	if _, err := io.ReadFull(reader, authResponse); err != nil {
		return err
	}
	if authResponse[1] != 0x00 {
		return fmt.Errorf("socks auth failed with status %d", authResponse[1])
	}

	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return err
	}
	request := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host).To4(); ip != nil {
		request = append(request, 0x01)
		request = append(request, ip...)
	} else {
		host = strings.Trim(host, "[]")
		request = append(request, 0x03, byte(len(host)))
		request = append(request, []byte(host)...)
	}
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	request = append(request, portBytes...)
	if _, err := conn.Write(request); err != nil {
		return err
	}

	reply := make([]byte, 10)
	if _, err := io.ReadFull(reader, reply); err != nil {
		return err
	}
	if reply[1] != 0x00 {
		return fmt.Errorf("socks connect failed with status %d", reply[1])
	}
	return nil
}
