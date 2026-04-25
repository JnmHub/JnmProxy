package model

type RuntimeNodeState struct {
	NodeID          int64  `json:"node_id"`
	FailureCount    int    `json:"failure_count"`
	CircuitOpen     bool   `json:"circuit_open"`
	CircuitUntil    string `json:"circuit_until,omitempty"`
	InCandidatePool bool   `json:"in_candidate_pool"`
	LastFailure     string `json:"last_failure,omitempty"`
	LastFailedAt    string `json:"last_failed_at,omitempty"`
}
