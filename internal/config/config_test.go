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
	if !cfg.SingBox.Enabled || cfg.SingBox.Version != "v1.13.8" || cfg.SingBox.Mode != "auto" {
		t.Fatalf("unexpected sing-box defaults: %#v", cfg.SingBox)
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
	t.Setenv("JNMPROXY_SING_BOX_MODE", "dialer")
	t.Setenv("JNMPROXY_SING_BOX_ENABLE_UDP", "true")

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
	if cfg.SingBox.Mode != "dialer" {
		t.Fatalf("unexpected sing-box mode: %s", cfg.SingBox.Mode)
	}
	if !cfg.SingBox.EnableUDP {
		t.Fatal("expected sing-box udp to be enabled by env")
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	cfg := Default()
	cfg.Stats.FlushIntervalSeconds = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRejectsInvalidSingBoxMode(t *testing.T) {
	cfg := Default()
	cfg.SingBox.Mode = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected sing-box validation error")
	}
}
