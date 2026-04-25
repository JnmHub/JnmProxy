package proxy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/repository"
)

type RequestAttempt struct {
	NodeID   int64  `json:"node_id"`
	NodeName string `json:"node_name"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

type RequestLogEvent struct {
	EntryProtocol    string
	CredentialID     int64
	Username         string
	TargetAddress    string
	Status           string
	SelectedNodeID   int64
	SelectedNodeName string
	Error            string
	Attempts         []RequestAttempt
	Duration         time.Duration
}

type RequestLogger struct {
	Repo             *repository.ProxyRequestLogRepository
	RecordFailedOnly bool
	WriteTimeout     time.Duration
}

func (logger *RequestLogger) Record(event RequestLogEvent) {
	if logger == nil || logger.Repo == nil {
		return
	}
	if logger.RecordFailedOnly && event.Status == "success" {
		return
	}
	attemptsJSON, err := json.Marshal(event.Attempts)
	if err != nil {
		attemptsJSON = []byte("[]")
	}
	timeout := logger.WriteTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = logger.Repo.Create(ctx, repository.CreateProxyRequestLogParams{
		EntryProtocol:    event.EntryProtocol,
		CredentialID:     event.CredentialID,
		Username:         event.Username,
		TargetAddress:    event.TargetAddress,
		Status:           event.Status,
		AttemptCount:     len(event.Attempts),
		SelectedNodeID:   event.SelectedNodeID,
		SelectedNodeName: event.SelectedNodeName,
		Error:            event.Error,
		AttemptsJSON:     string(attemptsJSON),
		DurationMS:       event.Duration.Milliseconds(),
	})
}

func markLastAttemptFailed(attempts []RequestAttempt, nodeID int64, err error) []RequestAttempt {
	if len(attempts) == 0 {
		return attempts
	}
	for index := len(attempts) - 1; index >= 0; index-- {
		if attempts[index].NodeID != nodeID {
			continue
		}
		attempts[index].Success = false
		if err != nil {
			attempts[index].Error = err.Error()
		}
		return attempts
	}
	return attempts
}
