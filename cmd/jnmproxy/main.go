package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/jnmproxy/jnmproxy/internal/config"
	"github.com/jnmproxy/jnmproxy/internal/db"
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

	logger.Info("server modules are not started yet; stage 1 only initializes configuration and database")
}
