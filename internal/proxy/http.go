package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/outbound"
	"github.com/jnmproxy/jnmproxy/internal/stats"
)

type HTTPProxy struct {
	Cache                 *cache.Store
	Dialer                *outbound.Dialer
	Stats                 *stats.Collector
	MaxAttemptsPerRequest int
}

func NewHTTPProxy(store *cache.Store, dialer *outbound.Dialer) *HTTPProxy {
	return &HTTPProxy{Cache: store, Dialer: dialer}
}

func (proxy *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := basicProxyCredential(r.Header.Get("Proxy-Authorization"))
	if !ok || !proxy.verifyCredential(username, password) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="JnmProxy"`)
		http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}

	if r.Method == http.MethodConnect {
		proxy.handleConnect(w, r, username)
		return
	}
	proxy.handleHTTP(w, r, username)
}

func (proxy *HTTPProxy) handleConnect(w http.ResponseWriter, r *http.Request, username string) {
	targetAddress := ensurePort(r.Host, "443")
	selection, outConn, err := proxy.dialWithRetries(r.Context(), username, targetAddress)
	if err != nil {
		http.Error(w, "connect upstream failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = outConn.Close()
		proxy.Cache.ReportNodeFailure(selection.Node.ID, "http hijacking not supported")
		recordStats(proxy.Stats, selection, 0, 0, false)
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		_ = outConn.Close()
		proxy.Cache.ReportNodeFailure(selection.Node.ID, err.Error())
		recordStats(proxy.Stats, selection, 0, 0, false)
		return
	}
	if _, err := io.WriteString(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		_ = clientConn.Close()
		_ = outConn.Close()
		proxy.Cache.ReportNodeFailure(selection.Node.ID, err.Error())
		recordStats(proxy.Stats, selection, 0, 0, false)
		return
	}
	uploadBytes, downloadBytes := pipeConnections(clientConn, outConn)
	proxy.Cache.ReportNodeSuccess(selection.Node.ID)
	recordStats(proxy.Stats, selection, uploadBytes, downloadBytes, true)
}

func (proxy *HTTPProxy) handleHTTP(w http.ResponseWriter, r *http.Request, username string) {
	targetAddress := httpTargetAddress(r)
	selection, outConn, err := proxy.dialWithRetries(r.Context(), username, targetAddress)
	if err != nil {
		http.Error(w, "connect upstream failed", http.StatusBadGateway)
		return
	}
	defer outConn.Close()

	outReq := new(http.Request)
	*outReq = *r
	outReq.RequestURI = ""
	outReq.URL = cloneURLForOriginRequest(r)
	outReq.Header = r.Header.Clone()
	outReq.Header.Del("Proxy-Authorization")
	outReq.Header.Del("Proxy-Connection")

	if err := outReq.Write(outConn); err != nil {
		proxy.Cache.ReportNodeFailure(selection.Node.ID, err.Error())
		recordStats(proxy.Stats, selection, requestUploadBytes(r), 0, false)
		http.Error(w, "write upstream request failed", http.StatusBadGateway)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(outConn), outReq)
	if err != nil {
		proxy.Cache.ReportNodeFailure(selection.Node.ID, err.Error())
		recordStats(proxy.Stats, selection, requestUploadBytes(r), 0, false)
		http.Error(w, "read upstream response failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	downloadBytes, _ := io.Copy(w, resp.Body)
	proxy.Cache.ReportNodeSuccess(selection.Node.ID)
	recordStats(proxy.Stats, selection, requestUploadBytes(r), downloadBytes, resp.StatusCode < 500)
}

func (proxy *HTTPProxy) dialWithRetries(ctx context.Context, username string, targetAddress string) (cache.Selection, net.Conn, error) {
	excluded := make(map[int64]struct{})
	var lastErr error
	for attempt := 0; attempt < normalizeMaxAttempts(proxy.MaxAttemptsPerRequest); attempt++ {
		selection, err := proxy.Cache.SelectExcluding(username, excluded)
		if err != nil {
			lastErr = err
			break
		}
		excluded[selection.Node.ID] = struct{}{}
		outConn, err := proxy.Dialer.DialContext(ctx, selection.Node, targetAddress)
		if err == nil {
			return selection, outConn, nil
		}
		lastErr = err
		proxy.Cache.ReportNodeFailure(selection.Node.ID, err.Error())
		recordStats(proxy.Stats, selection, 0, 0, false)
	}
	if lastErr == nil {
		lastErr = cache.ErrNoCandidateNodes
	}
	return cache.Selection{}, nil, lastErr
}

func (proxy *HTTPProxy) verifyCredential(username string, password string) bool {
	return verifyCachedCredential(proxy.Cache, username, password)
}

func basicProxyCredential(header string) (string, string, bool) {
	prefix := "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
	if err != nil {
		return "", "", false
	}
	username, password, ok := strings.Cut(string(decoded), ":")
	return username, password, ok
}

func httpTargetAddress(r *http.Request) string {
	host := r.URL.Host
	if host == "" {
		host = r.Host
	}
	defaultPort := "80"
	if strings.EqualFold(r.URL.Scheme, "https") {
		defaultPort = "443"
	}
	return ensurePort(host, defaultPort)
}

func ensurePort(host string, defaultPort string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, defaultPort)
}

func cloneURLForOriginRequest(r *http.Request) *url.URL {
	cloned := *r.URL
	cloned.Scheme = ""
	cloned.Host = ""
	return &cloned
}

func copyHeader(dst http.Header, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func pipeConnections(left net.Conn, right net.Conn) (int64, int64) {
	var closeOnce sync.Once
	closeBoth := func() {
		_ = left.Close()
		_ = right.Close()
	}
	type copyResult struct {
		leftToRight bool
		bytes       int64
	}
	done := make(chan copyResult, 2)
	go func() {
		bytes, _ := io.Copy(left, right)
		closeOnce.Do(closeBoth)
		done <- copyResult{leftToRight: false, bytes: bytes}
	}()
	go func() {
		bytes, _ := io.Copy(right, left)
		closeOnce.Do(closeBoth)
		done <- copyResult{leftToRight: true, bytes: bytes}
	}()
	first := <-done
	second := <-done

	var uploadBytes, downloadBytes int64
	for _, result := range []copyResult{first, second} {
		if result.leftToRight {
			uploadBytes += result.bytes
		} else {
			downloadBytes += result.bytes
		}
	}
	return uploadBytes, downloadBytes
}
