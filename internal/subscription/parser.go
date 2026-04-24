package subscription

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type ParsedNode struct {
	Name      string
	Protocol  string
	Server    string
	Port      int
	RawURI    string
	RawConfig map[string]any
}

func ParseNodes(content []byte) ([]ParsedNode, error) {
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return nil, errors.New("subscription body is empty")
	}

	if nodes, err := parseClashYAML([]byte(trimmed)); err == nil && len(nodes) > 0 {
		return nodes, nil
	}

	if decoded, err := decodeBase64Flexible(trimmed); err == nil {
		if nodes, err := parseURILines(string(decoded)); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	if nodes, err := parseURILines(trimmed); err == nil && len(nodes) > 0 {
		return nodes, nil
	}

	return nil, errors.New("unsupported subscription format or no proxy nodes found")
}

func parseClashYAML(content []byte) ([]ParsedNode, error) {
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Proxies) == 0 {
		return nil, errors.New("no clash proxies")
	}

	nodes := make([]ParsedNode, 0, len(cfg.Proxies))
	for _, proxy := range cfg.Proxies {
		node, err := nodeFromClashProxy(proxy)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, errors.New("no valid clash proxies")
	}
	return nodes, nil
}

func nodeFromClashProxy(proxy map[string]any) (ParsedNode, error) {
	name := stringField(proxy, "name")
	protocol := strings.ToLower(stringField(proxy, "type"))
	server := stringField(proxy, "server")
	port, ok := intField(proxy, "port")
	if name == "" || protocol == "" || server == "" || !ok {
		return ParsedNode{}, errors.New("missing required clash proxy fields")
	}

	return ParsedNode{
		Name:      name,
		Protocol:  normalizeProtocol(protocol),
		Server:    server,
		Port:      port,
		RawConfig: cloneMap(proxy),
	}, nil
}

func parseURILines(content string) ([]ParsedNode, error) {
	lines := strings.Split(content, "\n")
	nodes := make([]ParsedNode, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "://") {
			continue
		}
		node, err := ParseNodeURI(line)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, errors.New("no valid uri nodes")
	}
	return nodes, nil
}

func ParseNodeURI(rawURI string) (ParsedNode, error) {
	scheme := strings.ToLower(strings.TrimSpace(strings.SplitN(rawURI, "://", 2)[0]))
	switch scheme {
	case "vmess":
		return parseVMessURI(rawURI)
	case "ss":
		return parseShadowsocksURI(rawURI)
	case "trojan", "vless", "socks", "socks5", "socks5h", "http", "https":
		return parseGenericURI(rawURI, scheme)
	default:
		return ParsedNode{}, fmt.Errorf("unsupported uri scheme %q", scheme)
	}
}

func parseVMessURI(rawURI string) (ParsedNode, error) {
	payload := strings.TrimPrefix(rawURI, "vmess://")
	decoded, err := decodeBase64Flexible(payload)
	if err != nil {
		return ParsedNode{}, fmt.Errorf("decode vmess: %w", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(decoded, &cfg); err != nil {
		return ParsedNode{}, fmt.Errorf("parse vmess json: %w", err)
	}

	name := firstNonEmpty(stringField(cfg, "ps"), stringField(cfg, "name"), "vmess")
	server := firstNonEmpty(stringField(cfg, "add"), stringField(cfg, "server"))
	port, ok := intField(cfg, "port")
	if server == "" || !ok {
		return ParsedNode{}, errors.New("vmess missing server or port")
	}

	return ParsedNode{
		Name:      name,
		Protocol:  "vmess",
		Server:    server,
		Port:      port,
		RawURI:    rawURI,
		RawConfig: cfg,
	}, nil
}

func parseShadowsocksURI(rawURI string) (ParsedNode, error) {
	body := strings.TrimPrefix(rawURI, "ss://")
	fragment := ""
	if index := strings.Index(body, "#"); index >= 0 {
		fragment = body[index+1:]
		body = body[:index]
	}
	if index := strings.Index(body, "?"); index >= 0 {
		body = body[:index]
	}

	candidate := body
	if !strings.Contains(candidate, "@") {
		decoded, err := decodeBase64Flexible(candidate)
		if err != nil {
			return ParsedNode{}, fmt.Errorf("decode ss payload: %w", err)
		}
		candidate = string(decoded)
	} else {
		userInfo, hostPart, ok := strings.Cut(candidate, "@")
		if ok && !strings.Contains(userInfo, ":") {
			if decoded, err := decodeBase64Flexible(userInfo); err == nil {
				candidate = string(decoded) + "@" + hostPart
			}
		}
	}

	at := strings.LastIndex(candidate, "@")
	if at < 0 {
		return ParsedNode{}, errors.New("ss missing host separator")
	}
	hostPort := candidate[at+1:]
	server, port, err := splitHostPort(hostPort)
	if err != nil {
		return ParsedNode{}, err
	}

	name := decodeName(fragment)
	if name == "" {
		name = server
	}

	return ParsedNode{
		Name:     name,
		Protocol: "ss",
		Server:   server,
		Port:     port,
		RawURI:   rawURI,
		RawConfig: map[string]any{
			"uri":      rawURI,
			"identity": candidate[:at],
		},
	}, nil
}

func parseGenericURI(rawURI string, scheme string) (ParsedNode, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return ParsedNode{}, err
	}
	server := parsed.Hostname()
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return ParsedNode{}, fmt.Errorf("parse port: %w", err)
	}
	name := decodeName(parsed.Fragment)
	if name == "" {
		name = server
	}

	rawConfig := map[string]any{
		"scheme": scheme,
		"host":   server,
		"port":   port,
	}
	if parsed.User != nil {
		rawConfig["username"] = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			rawConfig["password"] = password
		}
	}
	if parsed.RawQuery != "" {
		rawConfig["query"] = parsed.RawQuery
	}

	return ParsedNode{
		Name:      name,
		Protocol:  normalizeProtocol(scheme),
		Server:    server,
		Port:      port,
		RawURI:    rawURI,
		RawConfig: rawConfig,
	}, nil
}

func decodeBase64Flexible(value string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', ' ':
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(value))
	if cleaned == "" {
		return nil, errors.New("empty base64 input")
	}
	padded := cleaned
	if remainder := len(padded) % 4; remainder != 0 {
		padded += strings.Repeat("=", 4-remainder)
	}

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(cleaned); err == nil {
			return decoded, nil
		}
		if decoded, err := encoding.DecodeString(padded); err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("invalid base64 input")
}

func splitHostPort(hostPort string) (string, int, error) {
	if strings.Count(hostPort, ":") > 1 && !strings.HasPrefix(hostPort, "[") {
		hostPort = "[" + hostPort + "]"
	}
	host, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", 0, fmt.Errorf("split host port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, fmt.Errorf("parse port: %w", err)
	}
	return strings.Trim(host, "[]"), port, nil
}

func normalizeProtocol(protocol string) string {
	switch strings.ToLower(protocol) {
	case "socks", "socks5h":
		return "socks5"
	default:
		return strings.ToLower(protocol)
	}
}

func decodeName(value string) string {
	if value == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func stringField(values map[string]any, key string) string {
	value, ok := values[key]
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

func intField(values map[string]any, key string) (int, bool) {
	value, ok := values[key]
	if !ok || value == nil {
		return 0, false
	}
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
		parsed, err := strconv.Atoi(fmt.Sprint(typed))
		return parsed, err == nil
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

func cloneMap(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
