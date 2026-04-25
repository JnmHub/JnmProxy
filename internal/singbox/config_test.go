package singbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jnmproxy/jnmproxy/internal/subscription"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
)

func TestBuildOutboundForCoreProtocols(t *testing.T) {
	cases := []struct {
		name          string
		uri           string
		wantType      string
		wantTransport string
	}{
		{
			name:     "shadowsocks",
			uri:      "ss://YWVzLTEyOC1nY206cGFzcw@hk.example.com:8388#HK",
			wantType: "shadowsocks",
		},
		{
			name:          "vless reality",
			uri:           "vless://00000000-0000-0000-0000-000000000000@vless.example.com:443?security=reality&pbk=public-key&sid=abcd&type=ws&path=%2Fws#VLESS",
			wantType:      "vless",
			wantTransport: "ws",
		},
		{
			name:          "trojan grpc",
			uri:           "trojan://secret@trojan.example.com:443?type=grpc&serviceName=svc&sni=trojan.example.com#Trojan",
			wantType:      "trojan",
			wantTransport: "grpc",
		},
		{
			name:     "hysteria2",
			uri:      "hysteria2://hy-pass@hy.example.com:443?sni=hy.example.com&obfs=salamander&obfs-password=obfs-pass#HY2",
			wantType: "hysteria2",
		},
		{
			name:     "tuic",
			uri:      "tuic://00000000-0000-0000-0000-000000000000:tuic-pass@tuic.example.com:443?congestion_control=bbr&udp_relay_mode=native&sni=tuic.example.com#TUIC",
			wantType: "tuic",
		},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := subscription.ParseNodeURI(tc.uri)
			if err != nil {
				t.Fatalf("parse uri: %v", err)
			}
			result := BuildOutbound(int64(index+1), node)
			if requiresQUIC(normalizeProtocol(node.Protocol)) && !QUICEnabled() {
				if result.Status != "error" {
					t.Fatalf("expected QUIC protocol to require with_quic, got %#v", result)
				}
				return
			}
			if result.Status != "supported" {
				t.Fatalf("expected supported result, got %#v", result)
			}
			assertValidOutboundJSON(t, result.JSON)

			var outbound map[string]any
			if err := json.Unmarshal([]byte(result.JSON), &outbound); err != nil {
				t.Fatalf("decode outbound json: %v", err)
			}
			if outbound["type"] != tc.wantType {
				t.Fatalf("expected type %s, got %#v json=%s", tc.wantType, outbound["type"], result.JSON)
			}
			if tc.wantTransport != "" && result.TransportType != tc.wantTransport {
				t.Fatalf("expected transport %s, got %s json=%s", tc.wantTransport, result.TransportType, result.JSON)
			}
		})
	}
}

func TestBuildOutboundForVMessClashYAML(t *testing.T) {
	nodes, err := subscription.ParseNodes([]byte(`
proxies:
  - name: "vmess ws tls"
    type: vmess
    server: vmess.example.com
    port: 443
    uuid: 00000000-0000-0000-0000-000000000000
    cipher: auto
    alterId: 0
    tls: true
    servername: vmess.example.com
    network: ws
    ws-opts:
      path: /ws
      host: edge.example.com
`))
	if err != nil {
		t.Fatalf("parse clash yaml: %v", err)
	}
	result := BuildOutbound(10, nodes[0])
	if result.Status != "supported" {
		t.Fatalf("expected supported result, got %#v", result)
	}
	if result.TransportType != "ws" {
		t.Fatalf("expected ws transport, got %s", result.TransportType)
	}
	assertValidOutboundJSON(t, result.JSON)
}

func TestBuildOutboundRejectsUnsupportedProtocol(t *testing.T) {
	result := BuildOutbound(1, subscription.ParsedNode{
		Name:     "unsupported",
		Protocol: "ssr",
		Server:   "example.com",
		Port:     1234,
	})
	if result.Status != "error" || result.Error == "" {
		t.Fatalf("expected error result, got %#v", result)
	}
}

func TestRedactJSON(t *testing.T) {
	redacted := RedactJSON(`{"type":"vless","uuid":"secret-id","tls":{"reality":{"public_key":"secret-key","short_id":"sid"}}}`)
	if redacted == "<invalid-json>" {
		t.Fatal("redaction returned invalid marker")
	}
	if redacted == `{"type":"vless","uuid":"secret-id","tls":{"reality":{"public_key":"secret-key","short_id":"sid"}}}` {
		t.Fatal("redaction did not change sensitive fields")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(redacted), &decoded); err != nil {
		t.Fatalf("decode redacted json: %v", err)
	}
	if decoded["uuid"] != "***" {
		t.Fatalf("uuid was not redacted: %s", redacted)
	}
}

func assertValidOutboundJSON(t *testing.T, rawJSON string) {
	t.Helper()
	ctx := include.Context(context.Background())
	var outbound option.Outbound
	if err := outbound.UnmarshalJSONContext(ctx, []byte(rawJSON)); err != nil {
		t.Fatalf("invalid sing-box outbound json: %v json=%s", err, rawJSON)
	}
}
