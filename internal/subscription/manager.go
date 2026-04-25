package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/repository"
)

type Manager struct {
	repo                 *repository.SubscriptionRepository
	client               *Client
	requestTimeout       time.Duration
	defaultUserAgent     string
	supportedProtocolSet map[string]struct{}
	singBoxBuilder       SingBoxOutboundBuilder
	nodeInvalidator      NodeInvalidator
}

type ManagerOptions struct {
	HTTPClient       *http.Client
	RequestTimeout   time.Duration
	DefaultUserAgent string
	SingBoxBuilder   SingBoxOutboundBuilder
	NodeInvalidator  NodeInvalidator
}

type RefreshResult struct {
	SubscriptionID        int64 `json:"subscription_id"`
	NodeCount             int   `json:"node_count"`
	HTTPStatus            int   `json:"http_status"`
	SingBoxSupportedCount int   `json:"sing_box_supported_count"`
	SingBoxErrorCount     int   `json:"sing_box_error_count"`
	UnsupportedCount      int   `json:"unsupported_count"`
}

type SingBoxBuildResult struct {
	JSON          string
	Status        string
	Error         string
	Version       string
	TransportType string
	UDPSupported  bool
}

type SingBoxOutboundBuilder func(node ParsedNode) SingBoxBuildResult

type NodeInvalidator func(nodeID int64)

func NewManager(repo *repository.SubscriptionRepository, opts ManagerOptions) *Manager {
	defaultUserAgent := opts.DefaultUserAgent
	if defaultUserAgent == "" {
		defaultUserAgent = DefaultUserAgent
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
		singBoxBuilder:   opts.SingBoxBuilder,
		nodeInvalidator:  opts.NodeInvalidator,
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

	fetchResult, parsedNodes, err := manager.fetchParsedNodes(ctx, subscription.URL, userAgent)
	if err != nil && fetchResult != nil && !strings.EqualFold(userAgent, DefaultUserAgent) {
		fallbackFetchResult, fallbackParsedNodes, fallbackErr := manager.fetchParsedNodes(ctx, subscription.URL, DefaultUserAgent)
		if fallbackErr == nil && !looksLikeUnsupportedClientNodes(fallbackParsedNodes) {
			fetchResult = fallbackFetchResult
			parsedNodes = fallbackParsedNodes
			err = nil
		}
	}
	if err != nil {
		finishedAt := time.Now().UTC()
		var httpStatus *int64
		if fetchResult != nil {
			status := int64(fetchResult.StatusCode)
			httpStatus = &status
		}
		recordErr := manager.repo.RecordRefreshResult(ctx, subscriptionID, repository.SubscriptionRefreshResult{
			Status:        "failed",
			HTTPStatus:    httpStatus,
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

	if looksLikeUnsupportedClientNodes(parsedNodes) && !strings.EqualFold(userAgent, DefaultUserAgent) {
		fallbackFetchResult, fallbackParsedNodes, fallbackErr := manager.fetchParsedNodes(ctx, subscription.URL, DefaultUserAgent)
		if fallbackErr == nil && !looksLikeUnsupportedClientNodes(fallbackParsedNodes) {
			fetchResult = fallbackFetchResult
			parsedNodes = fallbackParsedNodes
		}
	}
	if looksLikeUnsupportedClientNodes(parsedNodes) {
		err := errors.New("subscription returned unsupported-client placeholder nodes; set User-Agent to clash.meta")
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
	existingNodes, err := manager.repo.ListNodesBySubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	existingByKey := make(map[string]model.ProxyNode, len(existingNodes))
	for _, node := range existingNodes {
		existingByKey[node.SubscriptionNodeKey] = node
	}

	upsertNodes := make([]repository.UpsertProxyNodeParams, 0, len(parsedNodes))
	seenKeys := make([]string, 0, len(parsedNodes))
	seenKeySet := make(map[string]struct{}, len(parsedNodes))
	invalidatedNodeIDs := make(map[int64]struct{})
	var singBoxSupportedCount, singBoxErrorCount, unsupportedCount int
	for _, parsedNode := range parsedNodes {
		rawConfigJSON, err := marshalRawConfig(parsedNode)
		if err != nil {
			return nil, err
		}
		key := StableNodeKey(subscriptionID, parsedNode, rawConfigJSON)
		seenKeys = append(seenKeys, key)
		seenKeySet[key] = struct{}{}

		singBoxResult := manager.buildSingBoxOutbound(parsedNode)
		adapterStatus := manager.adapterStatus(parsedNode.Protocol, singBoxResult.Status)
		switch singBoxResult.Status {
		case "supported":
			singBoxSupportedCount++
		case "error":
			singBoxErrorCount++
		case "unsupported":
			if !manager.nativeSupported(parsedNode.Protocol) {
				unsupportedCount++
			}
		}

		upsertNode := repository.UpsertProxyNodeParams{
			SubscriptionID:      subscriptionID,
			SubscriptionNodeKey: key,
			Name:                parsedNode.Name,
			Protocol:            parsedNode.Protocol,
			Server:              parsedNode.Server,
			Port:                parsedNode.Port,
			RawURI:              parsedNode.RawURI,
			RawConfigJSON:       rawConfigJSON,
			SingBoxOutboundJSON: singBoxResult.JSON,
			SingBoxStatus:       singBoxResult.Status,
			SingBoxError:        singBoxResult.Error,
			SingBoxVersion:      singBoxResult.Version,
			UDPSupported:        singBoxResult.UDPSupported,
			TransportType:       singBoxResult.TransportType,
			AdapterStatus:       adapterStatus,
			LastSeenAt:          seenAt,
		}
		upsertNodes = append(upsertNodes, upsertNode)
		if existingNode, ok := existingByKey[key]; ok && nodeConfigChanged(existingNode, upsertNode) {
			invalidatedNodeIDs[existingNode.ID] = struct{}{}
		}
	}
	for _, existingNode := range existingNodes {
		if _, seen := seenKeySet[existingNode.SubscriptionNodeKey]; !seen && existingNode.Enabled {
			invalidatedNodeIDs[existingNode.ID] = struct{}{}
		}
	}

	if err := manager.repo.UpsertNodes(ctx, upsertNodes); err != nil {
		return nil, err
	}
	if err := manager.repo.MarkMissingNodes(ctx, subscriptionID, seenKeys); err != nil {
		return nil, err
	}
	manager.invalidateNodes(invalidatedNodeIDs)

	httpStatus := int64(fetchResult.StatusCode)
	if err := manager.repo.RecordRefreshResult(ctx, subscriptionID, repository.SubscriptionRefreshResult{
		Status:                "success",
		HTTPStatus:            &httpStatus,
		NodeCount:             len(parsedNodes),
		SingBoxSupportedCount: singBoxSupportedCount,
		SingBoxErrorCount:     singBoxErrorCount,
		UnsupportedCount:      unsupportedCount,
		StartedAt:             startedAtText,
		FinishedAt:            finishedAt.Format(time.RFC3339),
		LastRefreshAt:         finishedAt.Format(time.RFC3339),
		NextRefreshAt:         nextRefreshTime(finishedAt, subscription.RefreshIntervalSeconds),
		UploadBytes:           fetchResult.Usage.UploadBytes,
		DownloadBytes:         fetchResult.Usage.DownloadBytes,
		TotalBytes:            fetchResult.Usage.TotalBytes,
		ExpireAt:              fetchResult.Usage.ExpireAt,
	}); err != nil {
		return nil, err
	}

	return &RefreshResult{
		SubscriptionID:        subscriptionID,
		NodeCount:             len(parsedNodes),
		HTTPStatus:            fetchResult.StatusCode,
		SingBoxSupportedCount: singBoxSupportedCount,
		SingBoxErrorCount:     singBoxErrorCount,
		UnsupportedCount:      unsupportedCount,
	}, nil
}

func (manager *Manager) fetchParsedNodes(ctx context.Context, rawURL string, userAgent string) (*FetchResult, []ParsedNode, error) {
	fetchResult, err := manager.client.Fetch(ctx, rawURL, userAgent, manager.requestTimeout)
	if err != nil {
		return nil, nil, err
	}
	parsedNodes, err := ParseNodes(fetchResult.Body)
	if err != nil {
		return fetchResult, nil, err
	}
	return fetchResult, parsedNodes, nil
}

func StableNodeKey(subscriptionID int64, node ParsedNode, rawConfigJSON string) string {
	identity := fmt.Sprintf("%d|%s|%s|%d|%s|%s", subscriptionID, strings.ToLower(node.Protocol), node.Server, node.Port, node.Name, rawConfigJSON)
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func (manager *Manager) adapterStatus(protocol string, singBoxStatus string) string {
	if manager.nativeSupported(protocol) || singBoxStatus == "supported" {
		return "supported"
	}
	if singBoxStatus == "error" {
		return "error"
	}
	return "unsupported"
}

func (manager *Manager) nativeSupported(protocol string) bool {
	_, ok := manager.supportedProtocolSet[strings.ToLower(protocol)]
	return ok
}

func (manager *Manager) buildSingBoxOutbound(node ParsedNode) SingBoxBuildResult {
	if manager.singBoxBuilder == nil {
		return SingBoxBuildResult{Status: "unsupported"}
	}
	result := manager.singBoxBuilder(node)
	switch result.Status {
	case "supported", "unsupported", "error":
	default:
		result.Status = "unsupported"
	}
	if result.Status != "supported" {
		result.JSON = ""
		result.UDPSupported = false
		result.TransportType = ""
	}
	return result
}

func looksLikeUnsupportedClientNodes(nodes []ParsedNode) bool {
	if len(nodes) == 0 {
		return false
	}
	var hintCount, loopbackCount int
	for _, node := range nodes {
		name := strings.ToLower(node.Name)
		if strings.Contains(node.Name, "不支持") || strings.Contains(node.Name, "支持的代理软件") ||
			strings.Contains(name, "unsupported") || strings.Contains(name, "not support") {
			hintCount++
		}
		if isLoopbackServer(node.Server) {
			loopbackCount++
		}
	}
	return hintCount > 0 && loopbackCount == len(nodes)
}

func isLoopbackServer(server string) bool {
	if strings.EqualFold(server, "localhost") {
		return true
	}
	ip := net.ParseIP(server)
	return ip != nil && ip.IsLoopback()
}

func (manager *Manager) invalidateNodes(nodeIDs map[int64]struct{}) {
	if manager.nodeInvalidator == nil {
		return
	}
	for nodeID := range nodeIDs {
		manager.nodeInvalidator(nodeID)
	}
}

func nodeConfigChanged(existing model.ProxyNode, next repository.UpsertProxyNodeParams) bool {
	return existing.Protocol != next.Protocol ||
		existing.Server != next.Server ||
		existing.Port != next.Port ||
		existing.RawConfigJSON != next.RawConfigJSON ||
		existing.SingBoxOutboundJSON != next.SingBoxOutboundJSON ||
		string(existing.SingBoxStatus) != defaultString(next.SingBoxStatus, "unsupported") ||
		existing.SingBoxError != next.SingBoxError ||
		existing.SingBoxVersion != next.SingBoxVersion ||
		existing.UDPSupported != next.UDPSupported ||
		existing.TransportType != next.TransportType ||
		string(existing.AdapterStatus) != defaultString(next.AdapterStatus, "unsupported")
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
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
