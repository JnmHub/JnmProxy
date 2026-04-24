package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Proxy        ProxyConfig        `yaml:"proxy"`
	Database     DatabaseConfig     `yaml:"database"`
	Subscription SubscriptionConfig `yaml:"subscription"`
	Stats        StatsConfig        `yaml:"stats"`
	Scheduler    SchedulerConfig    `yaml:"scheduler"`
	Security     SecurityConfig     `yaml:"security"`
}

type ServerConfig struct {
	APIAddr string `yaml:"api_addr"`
}

type ProxyConfig struct {
	HTTPAddr  string `yaml:"http_addr"`
	SOCKSAddr string `yaml:"socks_addr"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type SubscriptionConfig struct {
	DefaultUserAgent              string `yaml:"default_user_agent"`
	DefaultRefreshIntervalSeconds int    `yaml:"default_refresh_interval_seconds"`
	RequestTimeoutSeconds         int    `yaml:"request_timeout_seconds"`
}

type StatsConfig struct {
	FlushIntervalSeconds int `yaml:"flush_interval_seconds"`
}

type SchedulerConfig struct {
	SubscriptionTickSeconds    int    `yaml:"subscription_tick_seconds"`
	HealthCheckIntervalSeconds int    `yaml:"health_check_interval_seconds"`
	HealthCheckTarget          string `yaml:"health_check_target"`
}

type SecurityConfig struct {
	RedactLogs bool `yaml:"redact_logs"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			APIAddr: "127.0.0.1:8080",
		},
		Proxy: ProxyConfig{
			HTTPAddr:  "127.0.0.1:1081",
			SOCKSAddr: "127.0.0.1:1080",
		},
		Database: DatabaseConfig{
			Path: "./data/jnmproxy.db",
		},
		Subscription: SubscriptionConfig{
			DefaultUserAgent:              "clash/1.18.0",
			DefaultRefreshIntervalSeconds: 3600,
			RequestTimeoutSeconds:         20,
		},
		Stats: StatsConfig{
			FlushIntervalSeconds: 10,
		},
		Scheduler: SchedulerConfig{
			SubscriptionTickSeconds:    30,
			HealthCheckIntervalSeconds: 300,
			HealthCheckTarget:          "",
		},
		Security: SecurityConfig{
			RedactLogs: true,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return Config{}, fmt.Errorf("read config %q: %w", path, err)
			}
		} else if err := yaml.Unmarshal(content, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %q: %w", path, err)
		}
	}

	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	if cfg.Server.APIAddr == "" {
		return errors.New("server.api_addr is required")
	}
	if cfg.Proxy.HTTPAddr == "" {
		return errors.New("proxy.http_addr is required")
	}
	if cfg.Proxy.SOCKSAddr == "" {
		return errors.New("proxy.socks_addr is required")
	}
	if cfg.Database.Path == "" {
		return errors.New("database.path is required")
	}
	if cfg.Subscription.DefaultUserAgent == "" {
		return errors.New("subscription.default_user_agent is required")
	}
	if cfg.Subscription.DefaultRefreshIntervalSeconds <= 0 {
		return errors.New("subscription.default_refresh_interval_seconds must be positive")
	}
	if cfg.Subscription.RequestTimeoutSeconds <= 0 {
		return errors.New("subscription.request_timeout_seconds must be positive")
	}
	if cfg.Stats.FlushIntervalSeconds <= 0 {
		return errors.New("stats.flush_interval_seconds must be positive")
	}
	if cfg.Scheduler.SubscriptionTickSeconds <= 0 {
		return errors.New("scheduler.subscription_tick_seconds must be positive")
	}
	if cfg.Scheduler.HealthCheckIntervalSeconds <= 0 {
		return errors.New("scheduler.health_check_interval_seconds must be positive")
	}
	return nil
}

func applyEnv(cfg *Config) error {
	setString := func(env string, target *string) {
		if value := os.Getenv(env); value != "" {
			*target = value
		}
	}
	setInt := func(env string, target *int) error {
		value := os.Getenv(env)
		if value == "" {
			return nil
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse %s: %w", env, err)
		}
		*target = parsed
		return nil
	}

	setString("JNMPROXY_API_ADDR", &cfg.Server.APIAddr)
	setString("JNMPROXY_HTTP_ADDR", &cfg.Proxy.HTTPAddr)
	setString("JNMPROXY_SOCKS_ADDR", &cfg.Proxy.SOCKSAddr)
	setString("JNMPROXY_DB_PATH", &cfg.Database.Path)
	setString("JNMPROXY_SUBSCRIPTION_DEFAULT_USER_AGENT", &cfg.Subscription.DefaultUserAgent)
	setString("JNMPROXY_HEALTH_CHECK_TARGET", &cfg.Scheduler.HealthCheckTarget)

	if err := setInt("JNMPROXY_SUBSCRIPTION_DEFAULT_REFRESH_INTERVAL_SECONDS", &cfg.Subscription.DefaultRefreshIntervalSeconds); err != nil {
		return err
	}
	if err := setInt("JNMPROXY_SUBSCRIPTION_REQUEST_TIMEOUT_SECONDS", &cfg.Subscription.RequestTimeoutSeconds); err != nil {
		return err
	}
	if err := setInt("JNMPROXY_STATS_FLUSH_INTERVAL_SECONDS", &cfg.Stats.FlushIntervalSeconds); err != nil {
		return err
	}
	if err := setInt("JNMPROXY_SCHEDULER_SUBSCRIPTION_TICK_SECONDS", &cfg.Scheduler.SubscriptionTickSeconds); err != nil {
		return err
	}
	if err := setInt("JNMPROXY_SCHEDULER_HEALTH_CHECK_INTERVAL_SECONDS", &cfg.Scheduler.HealthCheckIntervalSeconds); err != nil {
		return err
	}
	return nil
}
