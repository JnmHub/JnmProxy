package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/api"
	"github.com/jnmproxy/jnmproxy/internal/auth"
	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/config"
	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/grouping"
	"github.com/jnmproxy/jnmproxy/internal/outbound"
	proxyserver "github.com/jnmproxy/jnmproxy/internal/proxy"
	"github.com/jnmproxy/jnmproxy/internal/repository"
	"github.com/jnmproxy/jnmproxy/internal/scheduler"
	"github.com/jnmproxy/jnmproxy/internal/singbox"
	"github.com/jnmproxy/jnmproxy/internal/stats"
	"github.com/jnmproxy/jnmproxy/internal/subscription"
	"github.com/jnmproxy/jnmproxy/internal/webui"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	migrateOnly := flag.Bool("migrate-only", false, "run database migrations and exit")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	ctx := context.Background()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	store, err := db.Open(ctx, cfg.Database.Path)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := db.Migrate(ctx, store); err != nil {
		logger.Error("migrate database failed", "error", err)
		os.Exit(1)
	}

	logger.Info("jnmproxy backend initialized", "database", cfg.Database.Path)

	if *migrateOnly {
		return
	}

	runtimeCache := cache.NewStore()
	runtimeCache.ConfigureRuntime(cache.RuntimeOptions{
		FailureThreshold:     cfg.Runtime.FailureThreshold,
		CircuitBreakDuration: time.Duration(cfg.Runtime.CircuitBreakSeconds) * time.Second,
	})
	if err := runtimeCache.Load(ctx, store); err != nil {
		logger.Error("load runtime cache failed", "error", err)
		os.Exit(1)
	}

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	outboundDialer := outbound.NewDialer(30 * time.Second)
	var singBoxBuilder subscription.SingBoxOutboundBuilder
	var singBoxNodeInvalidator subscription.NodeInvalidator
	if cfg.SingBox.Enabled {
		singBoxAdapter := singbox.NewAdapter(singbox.AdapterOptions{
			MaxActiveEngines: cfg.SingBox.MaxActiveEngines,
			IdleTimeout:      time.Duration(cfg.SingBox.EngineIdleTimeoutSeconds) * time.Second,
			DialTimeout:      time.Duration(cfg.SingBox.EngineDialTimeoutSeconds) * time.Second,
			LogLevel:         cfg.SingBox.LogLevel,
		})
		outboundDialer.SingBox = singBoxAdapter
		singBoxBuilder = func(node subscription.ParsedNode) subscription.SingBoxBuildResult {
			result := singbox.BuildOutbound(0, node)
			return subscription.SingBoxBuildResult{
				JSON:          result.JSON,
				Status:        result.Status,
				Error:         result.Error,
				Version:       singbox.Version,
				TransportType: result.TransportType,
				UDPSupported:  result.UDPSupported,
			}
		}
		singBoxNodeInvalidator = func(nodeID int64) {
			if err := singBoxAdapter.CloseNode(nodeID); err != nil {
				logger.Warn("close sing-box node adapter failed", "node_id", nodeID, "error", err)
			}
		}
	}
	statsCollector := stats.NewCollector(time.Now)
	subscriptionRepo := repository.NewSubscriptionRepository(store)
	nodeRepo := repository.NewNodeRepository(store)
	groupRepo := repository.NewGroupRepository(store)
	credentialRepo := repository.NewCredentialRepository(store)
	healthRepo := repository.NewHealthRepository(store)
	statsRepo := repository.NewStatsRepository(store)
	operationLogRepo := repository.NewOperationLogRepository(store)
	proxyRequestLogRepo := repository.NewProxyRequestLogRepository(store)
	searchRepo := repository.NewSearchRepository(store)
	authService := auth.NewService(credentialRepo)
	groupingService := grouping.NewService(groupRepo)
	subscriptionManager := subscription.NewManager(subscriptionRepo, subscription.ManagerOptions{
		RequestTimeout:   time.Duration(cfg.Subscription.RequestTimeoutSeconds) * time.Second,
		DefaultUserAgent: cfg.Subscription.DefaultUserAgent,
		SingBoxBuilder:   singBoxBuilder,
		NodeInvalidator:  singBoxNodeInvalidator,
	})
	backgroundScheduler := &scheduler.Scheduler{
		DB:                  store,
		Cache:               runtimeCache,
		SubscriptionRepo:    subscriptionRepo,
		SubscriptionManager: subscriptionManager,
		HealthRepo:          healthRepo,
		HealthChecker:       scheduler.NewOutboundHealthChecker(outboundDialer, cfg.Scheduler.HealthCheckTarget, 10*time.Second),
		SubscriptionTick:    time.Duration(cfg.Scheduler.SubscriptionTickSeconds) * time.Second,
		HealthCheckInterval: time.Duration(cfg.Scheduler.HealthCheckIntervalSeconds) * time.Second,
		Logger:              logger,
	}
	apiHandler := &api.Server{
		DB:                  store,
		Cache:               runtimeCache,
		SubscriptionRepo:    subscriptionRepo,
		NodeRepo:            nodeRepo,
		GroupRepo:           groupRepo,
		CredentialRepo:      credentialRepo,
		HealthRepo:          healthRepo,
		StatsRepo:           statsRepo,
		OperationLogRepo:    operationLogRepo,
		ProxyRequestLogRepo: proxyRequestLogRepo,
		SearchRepo:          searchRepo,
		SubscriptionManager: subscriptionManager,
		AuthService:         authService,
		GroupingService:     groupingService,
		HealthChecker:       backgroundScheduler.HealthChecker,
		StatsCollector:      statsCollector,
		SingBoxStatus: &api.SingBoxStatus{
			Enabled:                  cfg.SingBox.Enabled,
			Version:                  singbox.Version,
			ConfigVersion:            cfg.SingBox.Version,
			Mode:                     cfg.SingBox.Mode,
			PreferNativeHTTPSOCKS:    cfg.SingBox.PreferNativeHTTPSOCKS,
			MaxActiveEngines:         cfg.SingBox.MaxActiveEngines,
			EngineIdleTimeoutSeconds: cfg.SingBox.EngineIdleTimeoutSeconds,
			EngineDialTimeoutSeconds: cfg.SingBox.EngineDialTimeoutSeconds,
			HealthCheckTarget:        cfg.SingBox.HealthCheckTarget,
			EnableUDP:                cfg.SingBox.EnableUDP,
			QUICEnabled:              singbox.QUICEnabled(),
			UTLSEnabled:              singbox.UTLSEnabled(),
			SupportedProtocols:       singbox.SupportedProtocols(),
			License:                  "GPL via github.com/sagernet/sing-box",
		},
		NodeAdapterInvalidator: singBoxNodeInvalidator,
		AdminToken:             cfg.Admin.Token,
	}
	webHandler, err := webui.NewHandler(apiHandler)
	if err != nil {
		logger.Error("initialize embedded web ui failed", "error", err)
		os.Exit(1)
	}
	apiServer := &http.Server{
		Addr:              cfg.Server.APIAddr,
		Handler:           webHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	httpProxyHandler := proxyserver.NewHTTPProxy(runtimeCache, outboundDialer)
	httpProxyHandler.Stats = statsCollector
	httpProxyHandler.MaxAttemptsPerRequest = cfg.Runtime.MaxAttemptsPerRequest
	if cfg.Runtime.RecordFailedRequests {
		httpProxyHandler.RequestLogger = &proxyserver.RequestLogger{Repo: proxyRequestLogRepo, RecordFailedOnly: true}
	}
	httpProxy := &http.Server{
		Addr:              cfg.Proxy.HTTPAddr,
		Handler:           httpProxyHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	socksListener, err := net.Listen("tcp", cfg.Proxy.SOCKSAddr)
	if err != nil {
		logger.Error("listen socks5 proxy failed", "addr", cfg.Proxy.SOCKSAddr, "error", err)
		os.Exit(1)
	}
	socksServer := proxyserver.NewSOCKS5Server(runtimeCache, outboundDialer)
	socksServer.Stats = statsCollector
	socksServer.MaxAttemptsPerRequest = cfg.Runtime.MaxAttemptsPerRequest
	if cfg.Runtime.RecordFailedRequests {
		socksServer.RequestLogger = &proxyserver.RequestLogger{Repo: proxyRequestLogRepo, RecordFailedOnly: true}
	}

	errCh := make(chan error, 3)
	go backgroundScheduler.Run(signalCtx)
	go func() {
		logger.Info("api server listening", "addr", cfg.Server.APIAddr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Stats.FlushIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-signalCtx.Done():
				return
			case <-ticker.C:
				if err := statsCollector.Flush(context.Background(), store); err != nil {
					logger.Error("flush stats failed", "error", err)
				}
			}
		}
	}()
	go func() {
		logger.Info("http proxy listening", "addr", cfg.Proxy.HTTPAddr)
		if err := httpProxy.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		logger.Info("socks5 proxy listening", "addr", cfg.Proxy.SOCKSAddr)
		if err := socksServer.Serve(socksListener); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("proxy server failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpProxy.Shutdown(shutdownCtx)
	_ = apiServer.Shutdown(shutdownCtx)
	_ = socksListener.Close()
	if err := statsCollector.Flush(context.Background(), store); err != nil {
		logger.Error("final stats flush failed", "error", err)
	}
}
