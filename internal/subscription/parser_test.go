package subscription

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParseUserInfoHeader(t *testing.T) {
	usage := ParseUserInfoHeader("upload=123; download=456; total=789; expire=1700000000")
	if usage.UploadBytes == nil || *usage.UploadBytes != 123 {
		t.Fatalf("unexpected upload: %#v", usage.UploadBytes)
	}
	if usage.DownloadBytes == nil || *usage.DownloadBytes != 456 {
		t.Fatalf("unexpected download: %#v", usage.DownloadBytes)
	}
	if usage.TotalBytes == nil || *usage.TotalBytes != 789 {
		t.Fatalf("unexpected total: %#v", usage.TotalBytes)
	}
	if usage.ExpireAt != "2023-11-14T22:13:20Z" {
		t.Fatalf("unexpected expire time: %s", usage.ExpireAt)
	}
}

func TestParseClashYAMLNodes(t *testing.T) {
	content := []byte(`
proxies:
  - name: "香港 01"
    type: ss
    server: hk.example.com
    port: 8388
    cipher: aes-128-gcm
    password: pass
  - name: "本地 Socks"
    type: socks5
    server: 127.0.0.1
    port: 1080
`)

	nodes, err := ParseNodes(content)
	if err != nil {
		t.Fatalf("ParseNodes returned error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Name != "香港 01" || nodes[0].Protocol != "ss" || nodes[0].Server != "hk.example.com" || nodes[0].Port != 8388 {
		t.Fatalf("unexpected first node: %#v", nodes[0])
	}
}

func TestParseBase64URINodes(t *testing.T) {
	vmessConfig := map[string]any{
		"ps":   "日本 01",
		"add":  "jp.example.com",
		"port": "443",
		"id":   "00000000-0000-0000-0000-000000000000",
	}
	vmessJSON, err := json.Marshal(vmessConfig)
	if err != nil {
		t.Fatalf("marshal vmess: %v", err)
	}
	body := "vmess://" + base64.StdEncoding.EncodeToString(vmessJSON) + "\n" +
		"trojan://pass@us.example.com:443#%E7%BE%8E%E5%9B%BD%2001\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(body))

	nodes, err := ParseNodes([]byte(encoded))
	if err != nil {
		t.Fatalf("ParseNodes returned error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Protocol != "vmess" || nodes[0].Name != "日本 01" {
		t.Fatalf("unexpected vmess node: %#v", nodes[0])
	}
	if nodes[1].Protocol != "trojan" || nodes[1].Name != "美国 01" {
		t.Fatalf("unexpected trojan node: %#v", nodes[1])
	}
}

func TestParseShadowsocksURI(t *testing.T) {
	identity := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:secret"))
	node, err := ParseNodeURI("ss://" + identity + "@hk.example.com:8388#HK")
	if err != nil {
		t.Fatalf("ParseNodeURI returned error: %v", err)
	}
	if node.Name != "HK" || node.Protocol != "ss" || node.Server != "hk.example.com" || node.Port != 8388 {
		t.Fatalf("unexpected node: %#v", node)
	}
}
