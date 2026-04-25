package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/auth"
	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/grouping"
	"github.com/jnmproxy/jnmproxy/internal/repository"
	"github.com/jnmproxy/jnmproxy/internal/scheduler"
	"github.com/jnmproxy/jnmproxy/internal/stats"
	"github.com/jnmproxy/jnmproxy/internal/subscription"
)

type Server struct {
	DB                     *sql.DB
	Cache                  *cache.Store
	SubscriptionRepo       *repository.SubscriptionRepository
	NodeRepo               *repository.NodeRepository
	GroupRepo              *repository.GroupRepository
	CredentialRepo         *repository.CredentialRepository
	HealthRepo             *repository.HealthRepository
	StatsRepo              *repository.StatsRepository
	SubscriptionManager    *subscription.Manager
	AuthService            *auth.Service
	GroupingService        *grouping.Service
	HealthChecker          scheduler.NodeChecker
	StatsCollector         *stats.Collector
	SingBoxStatus          *SingBoxStatus
	NodeAdapterInvalidator func(nodeID int64)
}

type SingBoxStatus struct {
	Enabled                  bool     `json:"enabled"`
	Version                  string   `json:"version"`
	ConfigVersion            string   `json:"config_version,omitempty"`
	Mode                     string   `json:"mode"`
	PreferNativeHTTPSOCKS    bool     `json:"prefer_native_http_socks"`
	AdapterConfigured        bool     `json:"adapter_configured"`
	MaxActiveEngines         int      `json:"max_active_engines"`
	EngineIdleTimeoutSeconds int      `json:"engine_idle_timeout_seconds"`
	EngineDialTimeoutSeconds int      `json:"engine_dial_timeout_seconds"`
	HealthCheckTarget        string   `json:"health_check_target,omitempty"`
	EnableUDP                bool     `json:"enable_udp"`
	SupportedProtocols       []string `json:"supported_protocols"`
	License                  string   `json:"license"`
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	segments := pathSegments(r.URL.Path)
	if len(segments) == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch segments[0] {
	case "system":
		server.handleSystem(w, r, segments)
	case "subscriptions":
		server.handleSubscriptions(w, r, segments)
	case "nodes":
		server.handleNodes(w, r, segments)
	case "groups":
		server.handleGroups(w, r, segments)
	case "group-keywords":
		server.handleGroupKeywords(w, r, segments)
	case "credentials":
		server.handleCredentials(w, r, segments)
	case "stats":
		server.handleStats(w, r, segments)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (server *Server) handleSystem(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 2 && segments[1] == "health" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
		return
	}
	if len(segments) == 2 && segments[1] == "sing-box" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, server.currentSingBoxStatus())
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (server *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request, segments []string) {
	ctx := r.Context()
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			items, err := server.SubscriptionRepo.List(ctx)
			writeResult(w, items, err)
		case http.MethodPost:
			var input subscriptionInput
			if !decodeRequest(w, r, &input) {
				return
			}
			enabled := true
			if input.Enabled != nil {
				enabled = *input.Enabled
			}
			item, err := server.SubscriptionRepo.Create(ctx, repository.CreateSubscriptionParams{
				Name:                   input.Name,
				URL:                    input.URL,
				UserAgent:              input.UserAgent,
				RefreshIntervalSeconds: input.RefreshIntervalSeconds,
				Enabled:                enabled,
			})
			server.reloadCache(ctx)
			writeResult(w, item, err)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	id, ok := parseID(w, segments[1])
	if !ok {
		return
	}
	if len(segments) == 2 {
		switch r.Method {
		case http.MethodGet:
			item, err := server.SubscriptionRepo.Get(ctx, id)
			writeResult(w, item, err)
		case http.MethodPut:
			var input subscriptionUpdateInput
			if !decodeRequest(w, r, &input) {
				return
			}
			item, err := server.SubscriptionRepo.Update(ctx, id, repository.UpdateSubscriptionParams{
				Name:                   input.Name,
				URL:                    input.URL,
				UserAgent:              input.UserAgent,
				RefreshIntervalSeconds: input.RefreshIntervalSeconds,
				Enabled:                input.Enabled,
			})
			server.reloadCache(ctx)
			writeResult(w, item, err)
		case http.MethodDelete:
			err := server.SubscriptionRepo.Delete(ctx, id)
			server.reloadCache(ctx)
			writeNoContent(w, err)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	if len(segments) == 3 {
		switch segments[2] {
		case "refresh":
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			result, err := server.SubscriptionManager.Refresh(ctx, id)
			server.reloadCache(ctx)
			writeResult(w, result, err)
		case "refresh-logs":
			if r.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			logs, err := server.SubscriptionRepo.ListRefreshLogs(ctx, id)
			writeResult(w, logs, err)
		case "nodes":
			if r.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			nodes, err := server.SubscriptionRepo.ListNodesBySubscription(ctx, id)
			writeResult(w, nodes, err)
		default:
			writeError(w, http.StatusNotFound, "not found")
		}
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (server *Server) handleNodes(w http.ResponseWriter, r *http.Request, segments []string) {
	ctx := r.Context()
	if len(segments) == 1 && r.Method == http.MethodGet {
		filter := repository.NodeListFilter{}
		filter.SubscriptionID = queryInt64(r, "subscription_id")
		filter.GroupID = queryInt64(r, "group_id")
		filter.Protocol = r.URL.Query().Get("protocol")
		filter.AliveStatus = r.URL.Query().Get("alive_status")
		if value := r.URL.Query().Get("enabled"); value != "" {
			enabled := value == "1" || strings.EqualFold(value, "true")
			filter.Enabled = &enabled
		}
		nodes, err := server.NodeRepo.List(ctx, filter)
		writeResult(w, nodes, err)
		return
	}
	if len(segments) == 2 && segments[1] == "batch" && r.Method == http.MethodPost {
		var input nodeBatchInput
		if !decodeRequest(w, r, &input) {
			return
		}
		err := server.handleNodeBatch(ctx, input)
		server.reloadCache(ctx)
		writeNoContent(w, err)
		return
	}
	if len(segments) == 2 && segments[1] == "check" && r.Method == http.MethodPost {
		count, err := server.checkAllNodes(ctx)
		server.reloadCache(ctx)
		writeResult(w, map[string]any{"checked": count}, err)
		return
	}
	if len(segments) < 2 {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, ok := parseID(w, segments[1])
	if !ok {
		return
	}
	if len(segments) == 2 {
		switch r.Method {
		case http.MethodGet:
			node, err := server.NodeRepo.Get(ctx, id)
			writeResult(w, node, err)
		case http.MethodPut:
			var input nodeUpdateInput
			if !decodeRequest(w, r, &input) {
				return
			}
			var err error
			if input.Enabled != nil {
				err = server.NodeRepo.SetEnabled(ctx, []int64{id}, *input.Enabled)
			}
			server.reloadCache(ctx)
			writeNoContent(w, err)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	if len(segments) == 3 && segments[2] == "check" && r.Method == http.MethodPost {
		err := server.checkNode(ctx, id)
		server.reloadCache(ctx)
		writeNoContent(w, err)
		return
	}
	if len(segments) == 3 && segments[2] == "rebuild-adapter" && r.Method == http.MethodPost {
		result, err := server.rebuildNodeAdapter(ctx, id)
		server.reloadCache(ctx)
		writeResult(w, result, err)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (server *Server) handleGroups(w http.ResponseWriter, r *http.Request, segments []string) {
	ctx := r.Context()
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			groups, err := server.GroupRepo.ListGroups(ctx)
			writeResult(w, groups, err)
		case http.MethodPost:
			var input groupInput
			if !decodeRequest(w, r, &input) {
				return
			}
			group, err := server.GroupRepo.CreateGroup(ctx, repository.CreateGroupParams{Name: input.Name, Description: input.Description, AutoCreated: input.AutoCreated})
			writeResult(w, group, err)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	id, ok := parseID(w, segments[1])
	if !ok {
		return
	}
	if len(segments) == 2 {
		switch r.Method {
		case http.MethodGet:
			group, err := server.GroupRepo.GetGroup(ctx, id)
			writeResult(w, group, err)
		case http.MethodPut:
			var input groupUpdateInput
			if !decodeRequest(w, r, &input) {
				return
			}
			group, err := server.GroupRepo.UpdateGroup(ctx, id, repository.UpdateGroupParams{Name: input.Name, Description: input.Description, AutoCreated: input.AutoCreated})
			writeResult(w, group, err)
		case http.MethodDelete:
			err := server.GroupRepo.DeleteGroup(ctx, id)
			server.reloadCache(ctx)
			writeNoContent(w, err)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	if len(segments) == 3 && segments[2] == "nodes" {
		var input groupNodesInput
		if !decodeRequest(w, r, &input) {
			return
		}
		var err error
		switch r.Method {
		case http.MethodPost:
			err = server.GroupRepo.AddNodesToGroup(ctx, id, input.NodeIDs)
		case http.MethodDelete:
			err = server.GroupRepo.RemoveNodesFromGroup(ctx, id, input.NodeIDs)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		server.reloadCache(ctx)
		writeNoContent(w, err)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (server *Server) handleGroupKeywords(w http.ResponseWriter, r *http.Request, segments []string) {
	ctx := r.Context()
	if len(segments) == 2 && segments[1] == "apply" && r.Method == http.MethodPost {
		var input applyKeywordsInput
		if !decodeRequest(w, r, &input) {
			return
		}
		result, err := server.GroupingService.ApplyKeywordGroups(ctx, grouping.ApplyKeywordParams{RuleIDs: input.RuleIDs, All: input.All})
		server.reloadCache(ctx)
		writeResult(w, result, err)
		return
	}
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			rules, err := server.GroupRepo.ListKeywordRules(ctx, false)
			writeResult(w, rules, err)
		case http.MethodPost:
			var input keywordInput
			if !decodeRequest(w, r, &input) {
				return
			}
			enabled := true
			if input.Enabled != nil {
				enabled = *input.Enabled
			}
			rule, err := server.GroupRepo.CreateKeywordRule(ctx, repository.CreateKeywordParams{
				Name: input.Name, Keywords: input.Keywords, CaseSensitive: input.CaseSensitive, Enabled: enabled,
			})
			writeResult(w, rule, err)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	id, ok := parseID(w, segments[1])
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input keywordUpdateInput
		if !decodeRequest(w, r, &input) {
			return
		}
		rule, err := server.GroupRepo.UpdateKeywordRule(ctx, id, repository.UpdateKeywordParams{
			Name: input.Name, Keywords: input.Keywords, CaseSensitive: input.CaseSensitive, Enabled: input.Enabled,
		})
		writeResult(w, rule, err)
	case http.MethodDelete:
		err := server.GroupRepo.DeleteKeywordRule(ctx, id)
		writeNoContent(w, err)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (server *Server) handleCredentials(w http.ResponseWriter, r *http.Request, segments []string) {
	ctx := r.Context()
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			items, err := server.CredentialRepo.List(ctx)
			writeResult(w, items, err)
		case http.MethodPost:
			var input credentialInput
			if !decodeRequest(w, r, &input) {
				return
			}
			enabled := true
			if input.Enabled != nil {
				enabled = *input.Enabled
			}
			item, err := server.AuthService.CreateCredential(ctx, auth.CreateCredentialInput{
				Username: input.Username, Password: input.Password, Enabled: enabled, BindMode: input.BindMode,
				SelectionPolicy: input.SelectionPolicy, Remark: input.Remark, Bindings: input.Bindings,
			})
			server.reloadCache(ctx)
			writeResult(w, item, err)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	id, ok := parseID(w, segments[1])
	if !ok {
		return
	}
	if len(segments) == 3 && segments[2] == "reset-password" && r.Method == http.MethodPost {
		var input resetPasswordInput
		if !decodeRequest(w, r, &input) {
			return
		}
		err := server.AuthService.ResetPassword(ctx, id, input.Password)
		server.reloadCache(ctx)
		writeNoContent(w, err)
		return
	}
	if len(segments) != 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := server.CredentialRepo.Get(ctx, id)
		writeResult(w, item, err)
	case http.MethodPut:
		var input credentialUpdateInput
		if !decodeRequest(w, r, &input) {
			return
		}
		item, err := server.AuthService.UpdateCredential(ctx, id, auth.UpdateCredentialInput{
			Enabled: input.Enabled, BindMode: input.BindMode, SelectionPolicy: input.SelectionPolicy,
			Remark: input.Remark, Bindings: input.Bindings,
		})
		server.reloadCache(ctx)
		writeResult(w, item, err)
	case http.MethodDelete:
		err := server.CredentialRepo.Delete(ctx, id)
		server.reloadCache(ctx)
		writeNoContent(w, err)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (server *Server) handleStats(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 2 && segments[1] == "overview" && r.Method == http.MethodGet {
		if server.StatsCollector != nil {
			_ = server.StatsCollector.Flush(r.Context(), server.DB)
		}
		overview, err := server.StatsRepo.Overview(r.Context())
		writeResult(w, overview, err)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (server *Server) handleNodeBatch(ctx context.Context, input nodeBatchInput) error {
	switch input.Action {
	case "enable":
		return server.NodeRepo.SetEnabled(ctx, input.NodeIDs, true)
	case "disable":
		return server.NodeRepo.SetEnabled(ctx, input.NodeIDs, false)
	case "add_group":
		return server.GroupRepo.AddNodesToGroup(ctx, input.GroupID, input.NodeIDs)
	case "remove_group":
		return server.GroupRepo.RemoveNodesFromGroup(ctx, input.GroupID, input.NodeIDs)
	default:
		return errors.New("unsupported batch action")
	}
}

func (server *Server) checkAllNodes(ctx context.Context) (int, error) {
	nodes, err := server.HealthRepo.ListCheckableNodes(ctx)
	if err != nil {
		return 0, err
	}
	for _, node := range nodes {
		result := server.HealthChecker.Check(ctx, node)
		result.NodeID = node.ID
		result.CheckedAt = time.Now().UTC().Format(time.RFC3339)
		if err := server.HealthRepo.RecordNodeHealth(ctx, result); err != nil {
			return 0, err
		}
	}
	return len(nodes), nil
}

func (server *Server) checkNode(ctx context.Context, id int64) error {
	node, err := server.NodeRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	result := server.HealthChecker.Check(ctx, *node)
	result.NodeID = id
	result.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	return server.HealthRepo.RecordNodeHealth(ctx, result)
}

func (server *Server) rebuildNodeAdapter(ctx context.Context, id int64) (map[string]any, error) {
	if _, err := server.NodeRepo.Get(ctx, id); err != nil {
		return nil, err
	}
	status := "adapter_not_configured"
	if server.NodeAdapterInvalidator != nil {
		server.NodeAdapterInvalidator(id)
		status = "adapter_rebuild_scheduled"
	}
	return map[string]any{"node_id": id, "status": status}, nil
}

func (server *Server) currentSingBoxStatus() SingBoxStatus {
	status := SingBoxStatus{
		Enabled:            false,
		AdapterConfigured:  server.NodeAdapterInvalidator != nil,
		SupportedProtocols: defaultSingBoxSupportedProtocols(),
		License:            "GPL via github.com/sagernet/sing-box",
	}
	if server.SingBoxStatus != nil {
		status = *server.SingBoxStatus
		status.AdapterConfigured = server.NodeAdapterInvalidator != nil
		if len(status.SupportedProtocols) == 0 {
			status.SupportedProtocols = defaultSingBoxSupportedProtocols()
		}
		if status.License == "" {
			status.License = "GPL via github.com/sagernet/sing-box"
		}
	}
	return status
}

func defaultSingBoxSupportedProtocols() []string {
	return []string{"ss", "shadowsocks", "vmess", "vless", "trojan", "hysteria2", "hy2", "tuic", "http", "https", "socks", "socks5", "socks5h"}
}

func (server *Server) reloadCache(ctx context.Context) {
	if server.Cache != nil && server.DB != nil {
		_ = server.Cache.Load(ctx, server.DB)
	}
}

type subscriptionInput struct {
	Name                   string `json:"name"`
	URL                    string `json:"url"`
	UserAgent              string `json:"user_agent"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	Enabled                *bool  `json:"enabled"`
}

type subscriptionUpdateInput struct {
	Name                   *string `json:"name"`
	URL                    *string `json:"url"`
	UserAgent              *string `json:"user_agent"`
	RefreshIntervalSeconds *int    `json:"refresh_interval_seconds"`
	Enabled                *bool   `json:"enabled"`
}

type nodeUpdateInput struct {
	Enabled *bool `json:"enabled"`
}

type nodeBatchInput struct {
	Action  string  `json:"action"`
	NodeIDs []int64 `json:"node_ids"`
	GroupID int64   `json:"group_id"`
}

type groupInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AutoCreated bool   `json:"auto_created"`
}

type groupUpdateInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	AutoCreated *bool   `json:"auto_created"`
}

type groupNodesInput struct {
	NodeIDs []int64 `json:"node_ids"`
}

type keywordInput struct {
	Name          string `json:"name"`
	Keywords      string `json:"keywords"`
	CaseSensitive bool   `json:"case_sensitive"`
	Enabled       *bool  `json:"enabled"`
}

type keywordUpdateInput struct {
	Name          *string `json:"name"`
	Keywords      *string `json:"keywords"`
	CaseSensitive *bool   `json:"case_sensitive"`
	Enabled       *bool   `json:"enabled"`
}

type applyKeywordsInput struct {
	RuleIDs []int64 `json:"rule_ids"`
	All     bool    `json:"all"`
}

type credentialInput struct {
	Username        string                               `json:"username"`
	Password        string                               `json:"password"`
	Enabled         *bool                                `json:"enabled"`
	BindMode        string                               `json:"bind_mode"`
	SelectionPolicy string                               `json:"selection_policy"`
	Remark          string                               `json:"remark"`
	Bindings        []repository.CredentialBindingTarget `json:"bindings"`
}

type credentialUpdateInput struct {
	Enabled         *bool                                 `json:"enabled"`
	BindMode        *string                               `json:"bind_mode"`
	SelectionPolicy *string                               `json:"selection_policy"`
	Remark          *string                               `json:"remark"`
	Bindings        *[]repository.CredentialBindingTarget `json:"bindings"`
}

type resetPasswordInput struct {
	Password string `json:"password"`
}

func pathSegments(path string) []string {
	path = strings.TrimPrefix(path, "/api/v1")
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func parseID(w http.ResponseWriter, value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func queryInt64(r *http.Request, key string) int64 {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func decodeRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeNoContent(w http.ResponseWriter, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
