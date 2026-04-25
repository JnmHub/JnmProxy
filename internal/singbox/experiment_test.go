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

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func TestEmbeddedBoxDirectOutboundDial(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("sing-box-direct-ok"))
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = include.Context(ctx)

	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, []byte(`{
		"log": {"disabled": true},
		"outbounds": [
			{"type": "direct", "tag": "direct-out"}
		],
		"route": {"final": "direct-out"}
	}`)); err != nil {
		t.Fatalf("unmarshal sing-box options: %v", err)
	}

	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatalf("create sing-box instance: %v", err)
	}
	defer instance.Close()

	if err := instance.Start(); err != nil {
		t.Fatalf("start sing-box instance: %v", err)
	}

	outbound, ok := instance.Outbound().Outbound("direct-out")
	if !ok {
		t.Fatal("direct outbound not found")
	}
	conn, err := outbound.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddr(targetURL.Host))
	if err != nil {
		t.Fatalf("dial through sing-box outbound: %v", err)
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
	if string(body) != "sing-box-direct-ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}
