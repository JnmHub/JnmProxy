package singbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/subscription"
	C "github.com/sagernet/sing-box/constant"
)

const Version = "v1.13.8"

type OutboundBuildResult struct {
	JSON          string
	Status        string
	Error         string
	TransportType string
	UDPSupported  bool
}

func BuildOutbound(nodeID int64, node subscription.ParsedNode) OutboundBuildResult {
	outbound, transportType, udpSupported, err := buildOutboundMap(nodeID, node)
	if err != nil {
		return OutboundBuildResult{
			Status: "error",
			Error:  err.Error(),
		}
	}
	content, err := json.Marshal(outbound)
	if err != nil {
		return OutboundBuildResult{
			Status: "error",
			Error:  fmt.Sprintf("marshal sing-box outbound: %v", err),
		}
	}
	return OutboundBuildResult{
		JSON:          string(content),
		Status:        "supported",
		TransportType: transportType,
		UDPSupported:  udpSupported,
	}
}

func buildOutboundMap(nodeID int64, node subscription.ParsedNode) (map[string]any, string, bool, error) {
	protocol := normalizeProtocol(node.Protocol)
	if requiresQUIC(protocol) && !QUICEnabled() {
		return nil, "", false, errors.New("sing-box QUIC support is not included; rebuild with -tags with_quic")
	}
	base := map[string]any{
		"type":        protocol,
		"tag":         fmt.Sprintf("node-%d", nodeID),
		"server":      node.Server,
		"server_port": node.Port,
	}
	if node.Server == "" || node.Port <= 0 {
		return nil, "", false, errors.New("node server and port are required")
	}

	raw := rawConfig(node)
	query := queryValues(raw)
	transport := buildTransport(raw, query)
	transportType := stringValue(transport["type"])
	tlsOptions := buildTLSOptions(protocol, raw, query)
	if requiresUTLS(tlsOptions) && !UTLSEnabled() {
		return nil, "", false, errors.New("sing-box uTLS support is not included; rebuild with -tags with_utls")
	}

	switch protocol {
	case "shadowsocks":
		method := firstNonEmpty(configString(raw, "method"), configString(raw, "cipher"), query.Get("method"), query.Get("cipher"))
		password := firstNonEmpty(configString(raw, "password"), query.Get("password"))
		if method == "" || password == "" {
			return nil, "", false, errors.New("shadowsocks method and password are required")
		}
		base["method"] = method
		base["password"] = password
		if plugin := configString(raw, "plugin"); plugin != "" {
			base["plugin"] = plugin
		}
		if pluginOpts := firstNonEmpty(configString(raw, "plugin_opts"), configString(raw, "plugin-opts")); pluginOpts != "" {
			base["plugin_opts"] = pluginOpts
		}
	case "vmess":
		uuid := firstNonEmpty(configString(raw, "uuid"), configString(raw, "id"), query.Get("id"))
		if uuid == "" {
			return nil, "", false, errors.New("vmess uuid is required")
		}
		base["uuid"] = uuid
		base["security"] = firstNonEmpty(configString(raw, "security"), configString(raw, "cipher"), query.Get("security"), "auto")
		if alterID, ok := configInt(raw, "alterId"); ok {
			base["alter_id"] = alterID
		} else if alterID, ok := configInt(raw, "alter_id"); ok {
			base["alter_id"] = alterID
		}
		if packetEncoding := firstNonEmpty(configString(raw, "packet_encoding"), query.Get("packetEncoding"), query.Get("packet_encoding")); packetEncoding != "" {
			base["packet_encoding"] = packetEncoding
		}
	case "vless":
		uuid := firstNonEmpty(configString(raw, "uuid"), configString(raw, "id"), configString(raw, "username"), query.Get("id"))
		if uuid == "" {
			return nil, "", false, errors.New("vless uuid is required")
		}
		base["uuid"] = uuid
		if flow := firstNonEmpty(configString(raw, "flow"), query.Get("flow")); flow != "" {
			base["flow"] = flow
		}
		if packetEncoding := firstNonEmpty(configString(raw, "packet_encoding"), query.Get("packetEncoding"), query.Get("packet_encoding")); packetEncoding != "" {
			base["packet_encoding"] = packetEncoding
		}
	case "trojan":
		password := firstNonEmpty(configString(raw, "password"), configString(raw, "username"), query.Get("password"))
		if password == "" {
			return nil, "", false, errors.New("trojan password is required")
		}
		base["password"] = password
	case "hysteria2":
		password := firstNonEmpty(configString(raw, "password"), configString(raw, "auth"), configString(raw, "username"), query.Get("password"), query.Get("auth"))
		if password == "" {
			return nil, "", false, errors.New("hysteria2 password is required")
		}
		base["password"] = password
		if up, ok := intFromAny(firstNonEmptyAny(raw["up_mbps"], raw["up"], query.Get("upmbps"), query.Get("up"))); ok {
			base["up_mbps"] = up
		}
		if down, ok := intFromAny(firstNonEmptyAny(raw["down_mbps"], raw["down"], query.Get("downmbps"), query.Get("down"))); ok {
			base["down_mbps"] = down
		}
		if obfsType := firstNonEmpty(configString(raw, "obfs"), query.Get("obfs")); obfsType != "" {
			base["obfs"] = map[string]any{
				"type":     obfsType,
				"password": firstNonEmpty(configString(raw, "obfs-password"), configString(raw, "obfs_password"), query.Get("obfs-password"), query.Get("obfs_password")),
			}
		}
	case "tuic":
		uuid := firstNonEmpty(configString(raw, "uuid"), configString(raw, "id"), configString(raw, "username"), query.Get("uuid"))
		password := firstNonEmpty(configString(raw, "password"), query.Get("password"))
		if uuid == "" || password == "" {
			return nil, "", false, errors.New("tuic uuid and password are required")
		}
		base["uuid"] = uuid
		base["password"] = password
		if value := firstNonEmpty(configString(raw, "congestion_control"), configString(raw, "congestion-controller"), query.Get("congestion_control"), query.Get("congestion-controller")); value != "" {
			base["congestion_control"] = value
		}
		if value := firstNonEmpty(configString(raw, "udp_relay_mode"), configString(raw, "udp-relay-mode"), query.Get("udp_relay_mode"), query.Get("udp-relay-mode")); value != "" {
			base["udp_relay_mode"] = value
		}
	case "http":
		if username := configString(raw, "username"); username != "" {
			base["username"] = username
		}
		if password := configString(raw, "password"); password != "" {
			base["password"] = password
		}
	case "socks":
		base["version"] = "5"
		if username := configString(raw, "username"); username != "" {
			base["username"] = username
		}
		if password := configString(raw, "password"); password != "" {
			base["password"] = password
		}
	default:
		return nil, "", false, fmt.Errorf("unsupported sing-box protocol %q", node.Protocol)
	}

	if len(tlsOptions) > 0 {
		base["tls"] = tlsOptions
	}
	if len(transport) > 0 {
		base["transport"] = transport
	}
	if network := firstNonEmpty(configString(raw, "network"), query.Get("network")); network == "udp" || network == "tcp" {
		base["network"] = network
	}

	return base, transportType, defaultUDPSupport(protocol, raw, query), nil
}

func QUICEnabled() bool {
	return C.WithQUIC
}

func SupportedProtocols() []string {
	protocols := []string{"ss", "shadowsocks", "vmess", "vless", "trojan", "http", "https", "socks", "socks5", "socks5h"}
	if QUICEnabled() {
		protocols = append(protocols, "hysteria2", "hy2", "tuic")
	}
	return protocols
}

func requiresQUIC(protocol string) bool {
	switch protocol {
	case "hysteria", "hysteria2", "tuic":
		return true
	default:
		return false
	}
}

func requiresUTLS(tlsOptions map[string]any) bool {
	if len(tlsOptions) == 0 {
		return false
	}
	if _, ok := tlsOptions["utls"]; ok {
		return true
	}
	if reality, ok := tlsOptions["reality"].(map[string]any); ok {
		return len(reality) > 0
	}
	return false
}

func rawConfig(node subscription.ParsedNode) map[string]any {
	if node.RawConfig == nil {
		return map[string]any{}
	}
	return node.RawConfig
}

func normalizeProtocol(protocol string) string {
	switch strings.ToLower(protocol) {
	case "ss", "shadowsocks":
		return "shadowsocks"
	case "hy2":
		return "hysteria2"
	case "socks", "socks5", "socks5h":
		return "socks"
	case "https":
		return "http"
	default:
		return strings.ToLower(protocol)
	}
}

func buildTLSOptions(protocol string, raw map[string]any, query url.Values) map[string]any {
	enabled := protocol == "trojan" || protocol == "hysteria2" || protocol == "tuic" || configBool(raw, "tls") || query.Get("security") == "tls" || query.Get("tls") == "1"
	if !enabled && query.Get("security") != "reality" {
		return nil
	}
	tlsOptions := map[string]any{"enabled": true}
	if serverName := firstNonEmpty(configString(raw, "servername"), configString(raw, "server_name"), configString(raw, "sni"), query.Get("sni"), query.Get("fp_server_name"), wsHostHeader(raw)); serverName != "" {
		tlsOptions["server_name"] = serverName
	}
	if configBool(raw, "skip-cert-verify") || configBool(raw, "skip_cert_verify") || query.Get("allowInsecure") == "1" {
		tlsOptions["insecure"] = true
	}
	if alpn := firstNonEmpty(configString(raw, "alpn"), query.Get("alpn")); alpn != "" {
		tlsOptions["alpn"] = splitCSV(alpn)
	}
	if fingerprint := firstNonEmpty(configString(raw, "client-fingerprint"), configString(raw, "client_fingerprint"), query.Get("fp")); fingerprint != "" {
		tlsOptions["utls"] = map[string]any{
			"enabled":     true,
			"fingerprint": fingerprint,
		}
	}
	if query.Get("security") == "reality" || mapValue(raw, "reality-opts") != nil || mapValue(raw, "reality_opts") != nil {
		reality := map[string]any{"enabled": true}
		realityOptions := firstMap(raw, "reality-opts", "reality_opts")
		if publicKey := firstNonEmpty(configString(realityOptions, "public-key"), configString(realityOptions, "public_key"), query.Get("pbk")); publicKey != "" {
			reality["public_key"] = publicKey
		}
		if shortID := firstNonEmpty(configString(realityOptions, "short-id"), configString(realityOptions, "short_id"), query.Get("sid")); shortID != "" {
			reality["short_id"] = shortID
		}
		tlsOptions["reality"] = reality
	}
	return tlsOptions
}

func buildTransport(raw map[string]any, query url.Values) map[string]any {
	network := strings.ToLower(firstNonEmpty(configString(raw, "network"), configString(raw, "net"), query.Get("type")))
	switch network {
	case "ws", "websocket":
		wsOptions := firstMap(raw, "ws-opts", "ws_opts")
		transport := map[string]any{"type": "ws"}
		if path := firstNonEmpty(configString(wsOptions, "path"), query.Get("path")); path != "" {
			transport["path"] = path
		}
		if host := firstNonEmpty(configString(wsOptions, "host"), wsHostHeader(raw), query.Get("host")); host != "" {
			transport["headers"] = map[string][]string{"Host": {host}}
		}
		return transport
	case "grpc":
		grpcOptions := firstMap(raw, "grpc-opts", "grpc_opts")
		transport := map[string]any{"type": "grpc"}
		if serviceName := firstNonEmpty(configString(grpcOptions, "grpc-service-name"), configString(grpcOptions, "service_name"), query.Get("serviceName"), query.Get("service_name")); serviceName != "" {
			transport["service_name"] = serviceName
		}
		return transport
	case "httpupgrade", "http_upgrade":
		return map[string]any{
			"type": "httpupgrade",
			"path": firstNonEmpty(query.Get("path"), "/"),
		}
	case "http":
		return map[string]any{
			"type": "http",
			"path": firstNonEmpty(query.Get("path"), "/"),
		}
	default:
		return nil
	}
}

func wsHostHeader(raw map[string]any) string {
	wsOptions := firstMap(raw, "ws-opts", "ws_opts")
	headers := firstMap(wsOptions, "headers")
	return firstNonEmpty(
		configString(headers, "Host"),
		configString(headers, "host"),
		configString(headers, ":authority"),
	)
}

func defaultUDPSupport(protocol string, raw map[string]any, query url.Values) bool {
	if value, ok := raw["udp"]; ok {
		return boolFromAny(value)
	}
	if udp := query.Get("udp"); udp != "" {
		parsed, _ := strconv.ParseBool(udp)
		return parsed
	}
	switch protocol {
	case "shadowsocks", "socks", "hysteria2", "tuic":
		return true
	default:
		return false
	}
}

func queryValues(raw map[string]any) url.Values {
	queryText := configString(raw, "query")
	if queryText == "" {
		return url.Values{}
	}
	values, err := url.ParseQuery(queryText)
	if err != nil {
		return url.Values{}
	}
	return values
}

func firstMap(raw map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value := mapValue(raw, key); value != nil {
			return value
		}
	}
	return map[string]any{}
}

func mapValue(raw map[string]any, key string) map[string]any {
	value, ok := raw[key]
	if !ok {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(map[any]any); ok {
		result := make(map[string]any, len(typed))
		for k, v := range typed {
			result[fmt.Sprint(k)] = v
		}
		return result
	}
	return nil
}

func configString(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func configInt(raw map[string]any, key string) (int, bool) {
	if raw == nil {
		return 0, false
	}
	return intFromAny(raw[key])
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func configBool(raw map[string]any, key string) bool {
	if raw == nil {
		return false
	}
	return boolFromAny(raw[key])
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		return parsed
	case int:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyAny(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		return value
	}
	return nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
