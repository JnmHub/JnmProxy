package model

type ProxyGroup struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AutoCreated bool   `json:"auto_created"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type GroupKeyword struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Keywords      string `json:"keywords"`
	CaseSensitive bool   `json:"case_sensitive"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type NodeName struct {
	ID   int64
	Name string
}
