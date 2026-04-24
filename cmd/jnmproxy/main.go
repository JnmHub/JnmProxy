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

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/config"
	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/outbound"
	proxyserver "github.com/jnmproxy/jnmproxy/internal/proxy"
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
	if err := runtimeCache.Load(ctx, store); err != nil {
		logger.Error("load runtime cache failed", "error", err)
		os.Exit(1)
	}

	outboundDialer := outbound.NewDialer(30 * time.Second)
	httpProxy := &http.Server{
		Addr:              cfg.Proxy.HTTPAddr,
		Handler:           proxyserver.NewHTTPProxy(runtimeCache, outboundDialer),
		ReadHeaderTimeout: 10 * time.Second,
	}
	socksListener, err := net.Listen("tcp", cfg.Proxy.SOCKSAddr)
	if err != nil {
		logger.Error("listen socks5 proxy failed", "addr", cfg.Proxy.SOCKSAddr, "error", err)
		os.Exit(1)
	}
	socksServer := proxyserver.NewSOCKS5Server(runtimeCache, outboundDialer)

	errCh := make(chan error, 2)
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

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("proxy server failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpProxy.Shutdown(shutdownCtx)
	_ = socksListener.Close()
}
