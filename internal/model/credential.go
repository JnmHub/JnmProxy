package model

type Credential struct {
	ID              int64              `json:"id"`
	Username        string             `json:"username"`
	PasswordHash    string             `json:"-"`
	Enabled         bool               `json:"enabled"`
	BindMode        CredentialBindMode `json:"bind_mode"`
	SelectionPolicy SelectionPolicy    `json:"selection_policy"`
	Remark          string             `json:"remark"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
}

type CredentialBinding struct {
	ID           int64  `json:"id"`
	CredentialID int64  `json:"credential_id"`
	TargetType   string `json:"target_type"`
	TargetID     int64  `json:"target_id"`
	CreatedAt    string `json:"created_at"`
}
