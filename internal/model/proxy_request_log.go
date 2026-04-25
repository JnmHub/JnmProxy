package model

type ProxyRequestLog struct {
	ID               int64  `json:"id"`
	EntryProtocol    string `json:"entry_protocol"`
	CredentialID     int64  `json:"credential_id"`
	Username         string `json:"username"`
	TargetAddress    string `json:"target_address"`
	Status           string `json:"status"`
	AttemptCount     int    `json:"attempt_count"`
	SelectedNodeID   int64  `json:"selected_node_id"`
	SelectedNodeName string `json:"selected_node_name"`
	Error            string `json:"error"`
	AttemptsJSON     string `json:"attempts_json"`
	DurationMS       int64  `json:"duration_ms"`
	CreatedAt        string `json:"created_at"`
}
