package scheduler

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/outbound"
	"github.com/jnmproxy/jnmproxy/internal/repository"
)

type NodeChecker interface {
	Check(ctx context.Context, node model.ProxyNode) repository.NodeHealthResult
}

type OutboundHealthChecker struct {
	Dialer        *outbound.Dialer
	TargetAddress string
	Timeout       time.Duration
}

func NewOutboundHealthChecker(dialer *outbound.Dialer, targetAddress string, timeout time.Duration) *OutboundHealthChecker {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &OutboundHealthChecker{
		Dialer:        dialer,
		TargetAddress: targetAddress,
		Timeout:       timeout,
	}
}

func (checker *OutboundHealthChecker) Check(ctx context.Context, node model.ProxyNode) repository.NodeHealthResult {
	start := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()

	var conn net.Conn
	var err error
	if checker.TargetAddress == "" {
		conn, err = (&net.Dialer{}).DialContext(checkCtx, "tcp", net.JoinHostPort(node.Server, strconv.Itoa(node.Port)))
	} else {
		dialer := checker.Dialer
		if dialer == nil {
			dialer = outbound.NewDialer(checker.Timeout)
		}
		conn, err = dialer.DialContext(checkCtx, cache.NodeSnapshot{
			ID:             node.ID,
			SubscriptionID: node.SubscriptionID,
			Name:           node.Name,
			Protocol:       node.Protocol,
			Server:         node.Server,
			Port:           node.Port,
			RawConfigJSON:  node.RawConfigJSON,
		}, checker.TargetAddress)
	}

	if err != nil {
		return repository.NodeHealthResult{
			Status: "dead",
			Error:  fmt.Sprintf("%v", err),
		}
	}
	_ = conn.Close()
	latency := time.Since(start).Milliseconds()
	return repository.NodeHealthResult{
		Status:    "alive",
		LatencyMS: &latency,
	}
}
