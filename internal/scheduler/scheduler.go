package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/repository"
	"github.com/jnmproxy/jnmproxy/internal/subscription"
)

type Scheduler struct {
	DB                  *sql.DB
	Cache               *cache.Store
	SubscriptionRepo    *repository.SubscriptionRepository
	SubscriptionManager *subscription.Manager
	HealthRepo          *repository.HealthRepository
	HealthChecker       NodeChecker
	SubscriptionTick    time.Duration
	HealthCheckInterval time.Duration
	Logger              *slog.Logger
	Now                 func() time.Time
}

func (scheduler *Scheduler) Run(ctx context.Context) {
	subscriptionTick := scheduler.SubscriptionTick
	if subscriptionTick <= 0 {
		subscriptionTick = 30 * time.Second
	}
	healthInterval := scheduler.HealthCheckInterval
	if healthInterval <= 0 {
		healthInterval = 5 * time.Minute
	}
	subscriptionTicker := time.NewTicker(subscriptionTick)
	healthTicker := time.NewTicker(healthInterval)
	defer subscriptionTicker.Stop()
	defer healthTicker.Stop()

	_, _ = scheduler.RunDueRefreshes(ctx)
	_, _ = scheduler.RunHealthChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-subscriptionTicker.C:
			if _, err := scheduler.RunDueRefreshes(ctx); err != nil {
				scheduler.logError("subscription refresh scheduler failed", err)
			}
		case <-healthTicker.C:
			if _, err := scheduler.RunHealthChecks(ctx); err != nil {
				scheduler.logError("health check scheduler failed", err)
			}
		}
	}
}

func (scheduler *Scheduler) RunDueRefreshes(ctx context.Context) (int, error) {
	if scheduler.SubscriptionRepo == nil || scheduler.SubscriptionManager == nil {
		return 0, nil
	}
	now := scheduler.now().UTC().Format(time.RFC3339)
	subscriptions, err := scheduler.SubscriptionRepo.ListDue(ctx, now)
	if err != nil {
		return 0, err
	}

	var joined error
	count := 0
	for _, item := range subscriptions {
		if _, err := scheduler.SubscriptionManager.Refresh(ctx, item.ID); err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		count++
	}
	if count > 0 {
		scheduler.reloadCache(ctx)
	}
	return count, joined
}

func (scheduler *Scheduler) RunHealthChecks(ctx context.Context) (int, error) {
	if scheduler.HealthRepo == nil || scheduler.HealthChecker == nil {
		return 0, nil
	}
	nodes, err := scheduler.HealthRepo.ListCheckableNodes(ctx)
	if err != nil {
		return 0, err
	}

	var joined error
	for _, node := range nodes {
		result := scheduler.HealthChecker.Check(ctx, node)
		result.NodeID = node.ID
		result.CheckedAt = scheduler.now().UTC().Format(time.RFC3339)
		if err := scheduler.HealthRepo.RecordNodeHealth(ctx, result); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	if len(nodes) > 0 {
		scheduler.reloadCache(ctx)
	}
	return len(nodes), joined
}

func (scheduler *Scheduler) reloadCache(ctx context.Context) {
	if scheduler.Cache == nil || scheduler.DB == nil {
		return
	}
	if err := scheduler.Cache.Load(ctx, scheduler.DB); err != nil {
		scheduler.logError("reload runtime cache failed", err)
	}
}

func (scheduler *Scheduler) logError(message string, err error) {
	if scheduler.Logger != nil && err != nil {
		scheduler.Logger.Error(message, "error", err)
	}
}

func (scheduler *Scheduler) now() time.Time {
	if scheduler.Now != nil {
		return scheduler.Now()
	}
	return time.Now()
}
