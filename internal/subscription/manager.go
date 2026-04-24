package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/repository"
)

type Manager struct {
	repo                 *repository.SubscriptionRepository
	client               *Client
	requestTimeout       time.Duration
	defaultUserAgent     string
	supportedProtocolSet map[string]struct{}
}

type ManagerOptions struct {
	HTTPClient       *http.Client
	RequestTimeout   time.Duration
	DefaultUserAgent string
}

type RefreshResult struct {
	SubscriptionID int64 `json:"subscription_id"`
	NodeCount      int   `json:"node_count"`
	HTTPStatus     int   `json:"http_status"`
}

func NewManager(repo *repository.SubscriptionRepository, opts ManagerOptions) *Manager {
	defaultUserAgent := opts.DefaultUserAgent
	if defaultUserAgent == "" {
		defaultUserAgent = "clash/1.18.0"
	}
	requestTimeout := opts.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 20 * time.Second
	}

	return &Manager{
		repo:             repo,
		client:           NewClient(opts.HTTPClient),
		requestTimeout:   requestTimeout,
		defaultUserAgent: defaultUserAgent,
		supportedProtocolSet: map[string]struct{}{
			"http":   {},
			"https":  {},
			"socks5": {},
		},
	}
}

func (manager *Manager) Refresh(ctx context.Context, subscriptionID int64) (*RefreshResult, error) {
	startedAt := time.Now().UTC()
	startedAtText := startedAt.Format(time.RFC3339)

	subscription, err := manager.repo.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	userAgent := subscription.UserAgent
	if userAgent == "" {
		userAgent = manager.defaultUserAgent
	}

	fetchResult, err := manager.client.Fetch(ctx, subscription.URL, userAgent, manager.requestTimeout)
	if err != nil {
		finishedAt := time.Now().UTC()
		recordErr := manager.repo.RecordRefreshResult(ctx, subscriptionID, repository.SubscriptionRefreshResult{
			Status:        "failed",
			Error:         err.Error(),
			StartedAt:     startedAtText,
			FinishedAt:    finishedAt.Format(time.RFC3339),
			LastRefreshAt: finishedAt.Format(time.RFC3339),
			NextRefreshAt: nextRefreshTime(finishedAt, subscription.RefreshIntervalSeconds),
		})
		if recordErr != nil {
			return nil, fmt.Errorf("%w; record refresh failure: %v", err, recordErr)
		}
		return nil, err
	}

	parsedNodes, err := ParseNodes(fetchResult.Body)
	if err != nil {
		finishedAt := time.Now().UTC()
		httpStatus := int64(fetchResult.StatusCode)
		recordErr := manager.repo.RecordRefreshResult(ctx, subscriptionID, repository.SubscriptionRefreshResult{
			Status:        "failed",
			HTTPStatus:    &httpStatus,
			Error:         err.Error(),
			StartedAt:     startedAtText,
			FinishedAt:    finishedAt.Format(time.RFC3339),
			LastRefreshAt: finishedAt.Format(time.RFC3339),
			NextRefreshAt: nextRefreshTime(finishedAt, subscription.RefreshIntervalSeconds),
		})
		if recordErr != nil {
			return nil, fmt.Errorf("%w; record refresh failure: %v", err, recordErr)
		}
		return nil, err
	}

	finishedAt := time.Now().UTC()
	seenAt := finishedAt.Format(time.RFC3339)
	upsertNodes := make([]repository.UpsertProxyNodeParams, 0, len(parsedNodes))
	seenKeys := make([]string, 0, len(parsedNodes))
	for _, parsedNode := range parsedNodes {
		rawConfigJSON, err := marshalRawConfig(parsedNode)
		if err != nil {
			return nil, err
		}
		key := StableNodeKey(subscriptionID, parsedNode, rawConfigJSON)
		seenKeys = append(seenKeys, key)
		upsertNodes = append(upsertNodes, repository.UpsertProxyNodeParams{
			SubscriptionID:      subscriptionID,
			SubscriptionNodeKey: key,
			Name:                parsedNode.Name,
			Protocol:            parsedNode.Protocol,
			Server:              parsedNode.Server,
			Port:                parsedNode.Port,
			RawURI:              parsedNode.RawURI,
			RawConfigJSON:       rawConfigJSON,
			AdapterStatus:       manager.adapterStatus(parsedNode.Protocol),
			LastSeenAt:          seenAt,
		})
	}

	if err := manager.repo.UpsertNodes(ctx, upsertNodes); err != nil {
		return nil, err
	}
	if err := manager.repo.MarkMissingNodes(ctx, subscriptionID, seenKeys); err != nil {
		return nil, err
	}

	httpStatus := int64(fetchResult.StatusCode)
	if err := manager.repo.RecordRefreshResult(ctx, subscriptionID, repository.SubscriptionRefreshResult{
		Status:        "success",
		HTTPStatus:    &httpStatus,
		NodeCount:     len(parsedNodes),
		StartedAt:     startedAtText,
		FinishedAt:    finishedAt.Format(time.RFC3339),
		LastRefreshAt: finishedAt.Format(time.RFC3339),
		NextRefreshAt: nextRefreshTime(finishedAt, subscription.RefreshIntervalSeconds),
		UploadBytes:   fetchResult.Usage.UploadBytes,
		DownloadBytes: fetchResult.Usage.DownloadBytes,
		TotalBytes:    fetchResult.Usage.TotalBytes,
		ExpireAt:      fetchResult.Usage.ExpireAt,
	}); err != nil {
		return nil, err
	}

	return &RefreshResult{
		SubscriptionID: subscriptionID,
		NodeCount:      len(parsedNodes),
		HTTPStatus:     fetchResult.StatusCode,
	}, nil
}

func StableNodeKey(subscriptionID int64, node ParsedNode, rawConfigJSON string) string {
	identity := fmt.Sprintf("%d|%s|%s|%d|%s|%s", subscriptionID, strings.ToLower(node.Protocol), node.Server, node.Port, node.Name, rawConfigJSON)
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func (manager *Manager) adapterStatus(protocol string) string {
	if _, ok := manager.supportedProtocolSet[strings.ToLower(protocol)]; ok {
		return "supported"
	}
	return "unsupported"
}

func marshalRawConfig(node ParsedNode) (string, error) {
	rawConfig := node.RawConfig
	if rawConfig == nil {
		rawConfig = map[string]any{}
	}
	content, err := json.Marshal(rawConfig)
	if err != nil {
		return "", fmt.Errorf("marshal node raw config %q: %w", node.Name, err)
	}
	return string(content), nil
}

func nextRefreshTime(now time.Time, intervalSeconds int) string {
	if intervalSeconds <= 0 {
		intervalSeconds = 3600
	}
	return now.Add(time.Duration(intervalSeconds) * time.Second).UTC().Format(time.RFC3339)
}
