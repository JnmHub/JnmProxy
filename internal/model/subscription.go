package model

type Subscription struct {
	ID                     int64              `json:"id"`
	Name                   string             `json:"name"`
	URL                    string             `json:"url"`
	UserAgent              string             `json:"user_agent"`
	RefreshIntervalSeconds int                `json:"refresh_interval_seconds"`
	Enabled                bool               `json:"enabled"`
	LastRefreshAt          string             `json:"last_refresh_at,omitempty"`
	NextRefreshAt          string             `json:"next_refresh_at,omitempty"`
	LastStatus             SubscriptionStatus `json:"last_status"`
	LastError              string             `json:"last_error,omitempty"`
	UploadBytes            *int64             `json:"upload_bytes,omitempty"`
	DownloadBytes          *int64             `json:"download_bytes,omitempty"`
	TotalBytes             *int64             `json:"total_bytes,omitempty"`
	ExpireAt               string             `json:"expire_at,omitempty"`
	CreatedAt              string             `json:"created_at"`
	UpdatedAt              string             `json:"updated_at"`
}

type ProxyNode struct {
	ID                  int64         `json:"id"`
	SubscriptionID      int64         `json:"subscription_id"`
	SubscriptionNodeKey string        `json:"subscription_node_key"`
	Name                string        `json:"name"`
	Protocol            string        `json:"protocol"`
	Server              string        `json:"server"`
	Port                int           `json:"port"`
	RawURI              string        `json:"raw_uri,omitempty"`
	RawConfigJSON       string        `json:"raw_config_json"`
	SingBoxOutboundJSON string        `json:"sing_box_outbound_json,omitempty"`
	SingBoxStatus       SingBoxStatus `json:"sing_box_status"`
	SingBoxError        string        `json:"sing_box_error,omitempty"`
	SingBoxVersion      string        `json:"sing_box_version,omitempty"`
	UDPSupported        bool          `json:"udp_supported"`
	TransportType       string        `json:"transport_type,omitempty"`
	AdapterStatus       AdapterStatus `json:"adapter_status"`
	Enabled             bool          `json:"enabled"`
	AliveStatus         AliveStatus   `json:"alive_status"`
	LastSeenAt          string        `json:"last_seen_at,omitempty"`
	LastCheckedAt       string        `json:"last_checked_at,omitempty"`
	LatencyMS           *int64        `json:"latency_ms,omitempty"`
	FailCount           int           `json:"fail_count"`
	GroupIDs            []int64       `json:"group_ids,omitempty"`
	CreatedAt           string        `json:"created_at"`
	UpdatedAt           string        `json:"updated_at"`
}

type SubscriptionRefreshLog struct {
	ID                    int64  `json:"id"`
	SubscriptionID        int64  `json:"subscription_id"`
	Status                string `json:"status"`
	HTTPStatus            *int64 `json:"http_status,omitempty"`
	NodeCount             int    `json:"node_count"`
	SingBoxSupportedCount int    `json:"sing_box_supported_count"`
	SingBoxErrorCount     int    `json:"sing_box_error_count"`
	UnsupportedCount      int    `json:"unsupported_count"`
	Error                 string `json:"error,omitempty"`
	StartedAt             string `json:"started_at"`
	FinishedAt            string `json:"finished_at,omitempty"`
	CreatedAt             string `json:"created_at"`
}
