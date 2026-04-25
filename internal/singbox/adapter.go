package singbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var supportedProtocols = map[string]struct{}{
	"ss":          {},
	"shadowsocks": {},
	"vmess":       {},
	"vless":       {},
	"trojan":      {},
	"hysteria2":   {},
	"hy2":         {},
	"tuic":        {},
	"hysteria":    {},
	"wireguard":   {},
	"shadowtls":   {},
	"anytls":      {},
	"naive":       {},
	"ssh":         {},
}

type Adapter struct {
	mu               sync.Mutex
	entries          map[int64]*engineEntry
	maxActiveEngines int
	idleTimeout      time.Duration
	dialTimeout      time.Duration
	logLevel         string
	now              func() time.Time
}

type AdapterOptions struct {
	MaxActiveEngines int
	IdleTimeout      time.Duration
	DialTimeout      time.Duration
	LogLevel         string
}

type engineEntry struct {
	configHash string
	instance   *box.Box
	outbound   adapter.Outbound
	lastUsed   time.Time
}

func NewAdapter(options AdapterOptions) *Adapter {
	if options.MaxActiveEngines <= 0 {
		options.MaxActiveEngines = 64
	}
	if options.IdleTimeout <= 0 {
		options.IdleTimeout = 10 * time.Minute
	}
	if options.DialTimeout <= 0 {
		options.DialTimeout = 30 * time.Second
	}
	if options.LogLevel == "" {
		options.LogLevel = "warn"
	}
	return &Adapter{
		entries:          make(map[int64]*engineEntry),
		maxActiveEngines: options.MaxActiveEngines,
		idleTimeout:      options.IdleTimeout,
		dialTimeout:      options.DialTimeout,
		logLevel:         options.LogLevel,
		now:              time.Now,
	}
}

func (adapter *Adapter) Supports(protocol string) bool {
	_, ok := supportedProtocols[strings.ToLower(protocol)]
	return ok
}

func (adapter *Adapter) DialContext(ctx context.Context, node cache.NodeSnapshot, targetAddress string) (net.Conn, error) {
	if node.SingBoxOutboundJSON == "" {
		return nil, errors.New("missing sing-box outbound json")
	}
	entry, err := adapter.engine(ctx, node)
	if err != nil {
		return nil, err
	}
	dialCtx := ctx
	var cancel context.CancelFunc
	if adapter.dialTimeout > 0 {
		dialCtx, cancel = context.WithTimeout(ctx, adapter.dialTimeout)
		defer cancel()
	}
	return entry.outbound.DialContext(dialCtx, N.NetworkTCP, M.ParseSocksaddr(targetAddress))
}

func (adapter *Adapter) CloseNode(nodeID int64) error {
	adapter.mu.Lock()
	entry := adapter.entries[nodeID]
	delete(adapter.entries, nodeID)
	adapter.mu.Unlock()
	if entry == nil || entry.instance == nil {
		return nil
	}
	return entry.instance.Close()
}

func (adapter *Adapter) CloseIdle() {
	now := adapter.now()
	var toClose []*box.Box
	adapter.mu.Lock()
	for nodeID, entry := range adapter.entries {
		if now.Sub(entry.lastUsed) <= adapter.idleTimeout {
			continue
		}
		toClose = append(toClose, entry.instance)
		delete(adapter.entries, nodeID)
	}
	adapter.mu.Unlock()
	for _, instance := range toClose {
		_ = instance.Close()
	}
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	entries := adapter.entries
	adapter.entries = make(map[int64]*engineEntry)
	adapter.mu.Unlock()

	var joined error
	for _, entry := range entries {
		if entry.instance != nil {
			joined = errors.Join(joined, entry.instance.Close())
		}
	}
	return joined
}

func (adapter *Adapter) engine(ctx context.Context, node cache.NodeSnapshot) (*engineEntry, error) {
	configHash := node.SingBoxOutboundJSON
	now := adapter.now()

	adapter.mu.Lock()
	if entry := adapter.entries[node.ID]; entry != nil && entry.configHash == configHash {
		entry.lastUsed = now
		adapter.mu.Unlock()
		return entry, nil
	}
	oldEntry := adapter.entries[node.ID]
	delete(adapter.entries, node.ID)
	adapter.mu.Unlock()

	if oldEntry != nil && oldEntry.instance != nil {
		_ = oldEntry.instance.Close()
	}

	entry, err := adapter.createEngine(ctx, node, configHash)
	if err != nil {
		return nil, err
	}
	entry.lastUsed = now

	adapter.mu.Lock()
	adapter.evictIfNeededLocked()
	adapter.entries[node.ID] = entry
	adapter.mu.Unlock()
	return entry, nil
}

func (adapter *Adapter) evictIfNeededLocked() {
	if len(adapter.entries) < adapter.maxActiveEngines {
		return
	}
	var oldestID int64
	var oldestAt time.Time
	for nodeID, entry := range adapter.entries {
		if oldestAt.IsZero() || entry.lastUsed.Before(oldestAt) {
			oldestID = nodeID
			oldestAt = entry.lastUsed
		}
	}
	if oldestID == 0 {
		return
	}
	entry := adapter.entries[oldestID]
	delete(adapter.entries, oldestID)
	go func() {
		if entry != nil && entry.instance != nil {
			_ = entry.instance.Close()
		}
	}()
}

func (adapter *Adapter) createEngine(ctx context.Context, node cache.NodeSnapshot, configHash string) (*engineEntry, error) {
	tag := fmt.Sprintf("node-%d", node.ID)
	rawConfig, err := boxConfigJSON(node.SingBoxOutboundJSON, tag, adapter.logLevel)
	if err != nil {
		return nil, err
	}
	boxCtx := include.Context(ctx)
	var options option.Options
	if err := options.UnmarshalJSONContext(boxCtx, rawConfig); err != nil {
		return nil, fmt.Errorf("unmarshal sing-box options: %w", err)
	}
	instance, err := box.New(box.Options{Context: boxCtx, Options: options})
	if err != nil {
		return nil, fmt.Errorf("create sing-box instance: %w", err)
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		return nil, fmt.Errorf("start sing-box instance: %w", err)
	}
	outbound, ok := instance.Outbound().Outbound(tag)
	if !ok {
		_ = instance.Close()
		return nil, fmt.Errorf("sing-box outbound %q not found", tag)
	}
	return &engineEntry{
		configHash: configHash,
		instance:   instance,
		outbound:   outbound,
	}, nil
}

func boxConfigJSON(outboundJSON string, tag string, logLevel string) ([]byte, error) {
	var outbound map[string]any
	if err := json.Unmarshal([]byte(outboundJSON), &outbound); err != nil {
		return nil, fmt.Errorf("decode sing-box outbound json: %w", err)
	}
	outbound["tag"] = tag
	config := map[string]any{
		"log": map[string]any{
			"disabled": logLevel == "disabled",
			"level":    logLevel,
		},
		"outbounds": []any{outbound},
		"route": map[string]any{
			"final": tag,
		},
	}
	return json.Marshal(config)
}
