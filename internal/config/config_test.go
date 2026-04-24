package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Database.Path != "./data/jnmproxy.db" {
		t.Fatalf("unexpected database path: %s", cfg.Database.Path)
	}
	if cfg.Subscription.DefaultUserAgent != "clash/1.18.0" {
		t.Fatalf("unexpected user agent: %s", cfg.Subscription.DefaultUserAgent)
	}
}

func TestLoadConfigAndEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
server:
  api_addr: "127.0.0.1:18080"
database:
  path: "./custom.db"
subscription:
  default_refresh_interval_seconds: 7200
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("JNMPROXY_DB_PATH", "./env.db")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.APIAddr != "127.0.0.1:18080" {
		t.Fatalf("unexpected api addr: %s", cfg.Server.APIAddr)
	}
	if cfg.Database.Path != "./env.db" {
		t.Fatalf("unexpected database path: %s", cfg.Database.Path)
	}
	if cfg.Subscription.DefaultRefreshIntervalSeconds != 7200 {
		t.Fatalf("unexpected refresh interval: %d", cfg.Subscription.DefaultRefreshIntervalSeconds)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	cfg := Default()
	cfg.Stats.FlushIntervalSeconds = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
