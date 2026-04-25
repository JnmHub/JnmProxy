package singbox

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/subscription"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
)

func TestCoreProtocolTCPTransfers(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("through-" + r.URL.Query().Get("case")))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}

	cases := []struct {
		name         string
		inbound      map[string]any
		node         subscription.ParsedNode
		expectedBody string
	}{
		{
			name: "shadowsocks",
			inbound: map[string]any{
				"type":     "shadowsocks",
				"method":   "aes-128-gcm",
				"password": "test-password",
			},
			node: subscription.ParsedNode{
				Name:     "ss",
				Protocol: "ss",
				RawConfig: map[string]any{
					"method":   "aes-128-gcm",
					"password": "test-password",
				},
			},
			expectedBody: "through-shadowsocks",
		},
		{
			name: "vmess",
			inbound: map[string]any{
				"type": "vmess",
				"users": []map[string]any{
					{"uuid": "00000000-0000-0000-0000-000000000001", "alterId": 0},
				},
			},
			node: subscription.ParsedNode{
				Name:     "vmess",
				Protocol: "vmess",
				RawConfig: map[string]any{
					"uuid":     "00000000-0000-0000-0000-000000000001",
					"security": "auto",
				},
			},
			expectedBody: "through-vmess",
		},
		{
			name: "vless",
			inbound: map[string]any{
				"type": "vless",
				"users": []map[string]any{
					{"uuid": "00000000-0000-0000-0000-000000000002"},
				},
			},
			node: subscription.ParsedNode{
				Name:     "vless",
				Protocol: "vless",
				RawConfig: map[string]any{
					"uuid": "00000000-0000-0000-0000-000000000002",
				},
			},
			expectedBody: "through-vless",
		},
		{
			name: "trojan",
			inbound: map[string]any{
				"type": "trojan",
				"users": []map[string]any{
					{"password": "trojan-pass"},
				},
				"tls": selfSignedInboundTLS(t),
			},
			node: subscription.ParsedNode{
				Name:     "trojan",
				Protocol: "trojan",
				RawConfig: map[string]any{
					"password":         "trojan-pass",
					"sni":              "localhost",
					"skip-cert-verify": true,
				},
			},
			expectedBody: "through-trojan",
		},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			port := freeTCPPort(t)
			instance := startProtocolServer(t, port, tc.inbound)
			defer instance.Close()

			tc.node.Server = "127.0.0.1"
			tc.node.Port = port
			result := BuildOutbound(int64(index+1), tc.node)
			if result.Status != "supported" {
				t.Fatalf("build outbound: %#v", result)
			}
			adapter := NewAdapter(AdapterOptions{
				MaxActiveEngines: 2,
				IdleTimeout:      time.Minute,
				DialTimeout:      5 * time.Second,
				LogLevel:         "disabled",
			})
			defer adapter.Close()
			conn, err := adapter.DialContext(context.Background(), cache.NodeSnapshot{
				ID:                  int64(index + 1),
				Protocol:            tc.node.Protocol,
				SingBoxStatus:       model.SingBoxStatusSupported,
				SingBoxOutboundJSON: result.JSON,
			}, targetURL.Host)
			if err != nil {
				t.Fatalf("dial via protocol: %v", err)
			}
			defer conn.Close()

			path := "/?case=" + url.QueryEscape(tc.name)
			if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", path, targetURL.Host); err != nil {
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
			if string(body) != tc.expectedBody {
				t.Fatalf("expected %q, got %q", tc.expectedBody, string(body))
			}
		})
	}
}

func TestQUICProtocolOutboundConfigsRemainValid(t *testing.T) {
	nodes := []subscription.ParsedNode{
		{
			Name:     "hy2",
			Protocol: "hysteria2",
			Server:   "hy.example.com",
			Port:     443,
			RawConfig: map[string]any{
				"password": "hy-pass",
				"sni":      "hy.example.com",
			},
		},
		{
			Name:     "tuic",
			Protocol: "tuic",
			Server:   "tuic.example.com",
			Port:     443,
			RawConfig: map[string]any{
				"uuid":     "00000000-0000-0000-0000-000000000003",
				"password": "tuic-pass",
				"sni":      "tuic.example.com",
			},
		},
	}
	for index, node := range nodes {
		result := BuildOutbound(int64(index+100), node)
		if result.Status != "supported" {
			t.Fatalf("build outbound for %s: %#v", node.Protocol, result)
		}
		assertValidOutboundJSON(t, result.JSON)
	}
}

func startProtocolServer(t *testing.T, port int, inbound map[string]any) *box.Box {
	t.Helper()
	inbound["tag"] = "in"
	inbound["listen"] = "127.0.0.1"
	inbound["listen_port"] = port
	rawConfig, err := json.Marshal(map[string]any{
		"log": map[string]any{"disabled": true},
		"inbounds": []any{
			inbound,
		},
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"route": map[string]any{"final": "direct"},
	})
	if err != nil {
		t.Fatalf("marshal server config: %v", err)
	}

	ctx := include.Context(context.Background())
	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, rawConfig); err != nil {
		t.Fatalf("unmarshal server config: %v config=%s", err, string(rawConfig))
	}
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatalf("create server box: %v", err)
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		t.Fatalf("start server box: %v", err)
	}
	return instance
}

func selfSignedInboundTLS(t *testing.T) map[string]any {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate tls key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		DNSNames:              []string{"localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("generate tls certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return map[string]any{
		"enabled":     true,
		"server_name": "localhost",
		"certificate": string(certPEM),
		"key":         string(keyPEM),
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
