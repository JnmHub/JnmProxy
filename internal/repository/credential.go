package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

type CredentialRepository struct {
	db *sql.DB
}

func NewCredentialRepository(db *sql.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

type CreateCredentialParams struct {
	Username        string
	PasswordHash    string
	Enabled         bool
	BindMode        string
	SelectionPolicy string
	Remark          string
	Bindings        []CredentialBindingTarget
}

type UpdateCredentialParams struct {
	Enabled         *bool
	BindMode        *string
	SelectionPolicy *string
	Remark          *string
	Bindings        *[]CredentialBindingTarget
}

type CredentialBindingTarget struct {
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
}

type StickyState struct {
	CredentialID int64
	NodeID       int64
	UpdatedAt    string
}

func (repo *CredentialRepository) Create(ctx context.Context, params CreateCredentialParams) (*model.Credential, error) {
	username := strings.TrimSpace(params.Username)
	if username == "" {
		return nil, errors.New("username is required")
	}
	if params.PasswordHash == "" {
		return nil, errors.New("password hash is required")
	}
	bindMode := defaultString(params.BindMode, "all")
	selectionPolicy := defaultString(params.SelectionPolicy, "random")

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create credential: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
INSERT INTO credentials (username, password_hash, enabled, bind_mode, selection_policy, remark)
VALUES (?, ?, ?, ?, ?, ?)
`, username, params.PasswordHash, boolToInt(params.Enabled), bindMode, selectionPolicy, params.Remark)
	if err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read credential id: %w", err)
	}
	if err := replaceCredentialBindings(ctx, tx, id, params.Bindings); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create credential: %w", err)
	}
	return repo.Get(ctx, id)
}

func (repo *CredentialRepository) Get(ctx context.Context, id int64) (*model.Credential, error) {
	row := repo.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, enabled, bind_mode, selection_policy, remark, created_at, updated_at
FROM credentials
WHERE id = ?
`, id)
	return scanCredential(row)
}

func (repo *CredentialRepository) GetByUsername(ctx context.Context, username string) (*model.Credential, error) {
	row := repo.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, enabled, bind_mode, selection_policy, remark, created_at, updated_at
FROM credentials
WHERE username = ?
`, username)
	return scanCredential(row)
}

func (repo *CredentialRepository) List(ctx context.Context) ([]model.Credential, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, username, password_hash, enabled, bind_mode, selection_policy, remark, created_at, updated_at
FROM credentials
ORDER BY id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var credentials []model.Credential
	for rows.Next() {
		credential, err := scanCredential(rows)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, *credential)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credentials: %w", err)
	}
	return credentials, nil
}

func (repo *CredentialRepository) Update(ctx context.Context, id int64, params UpdateCredentialParams) (*model.Credential, error) {
	current, err := repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if params.Enabled != nil {
		current.Enabled = *params.Enabled
	}
	if params.BindMode != nil {
		current.BindMode = model.CredentialBindMode(*params.BindMode)
	}
	if params.SelectionPolicy != nil {
		current.SelectionPolicy = model.SelectionPolicy(*params.SelectionPolicy)
	}
	if params.Remark != nil {
		current.Remark = *params.Remark
	}

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update credential: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE credentials
SET enabled = ?, bind_mode = ?, selection_policy = ?, remark = ?, updated_at = datetime('now')
WHERE id = ?
`, boolToInt(current.Enabled), current.BindMode, current.SelectionPolicy, current.Remark, id); err != nil {
		return nil, fmt.Errorf("update credential: %w", err)
	}
	if params.Bindings != nil {
		if err := replaceCredentialBindings(ctx, tx, id, *params.Bindings); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit update credential: %w", err)
	}
	return repo.Get(ctx, id)
}

func (repo *CredentialRepository) ResetPassword(ctx context.Context, id int64, passwordHash string) error {
	if passwordHash == "" {
		return errors.New("password hash is required")
	}
	if _, err := repo.db.ExecContext(ctx, `
UPDATE credentials
SET password_hash = ?, updated_at = datetime('now')
WHERE id = ?
`, passwordHash, id); err != nil {
		return fmt.Errorf("reset credential password: %w", err)
	}
	return nil
}

func (repo *CredentialRepository) Delete(ctx context.Context, id int64) error {
	if _, err := repo.db.ExecContext(ctx, "DELETE FROM credentials WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}

func (repo *CredentialRepository) SetStickyNode(ctx context.Context, credentialID int64, nodeID int64) error {
	if credentialID <= 0 {
		return errors.New("credential id must be positive")
	}
	if nodeID <= 0 {
		return errors.New("sticky node id must be positive")
	}
	if _, err := repo.db.ExecContext(ctx, `
INSERT INTO credential_sticky_states (credential_id, node_id, updated_at)
VALUES (?, ?, datetime('now'))
ON CONFLICT(credential_id) DO UPDATE SET
	node_id = excluded.node_id,
	updated_at = datetime('now')
`, credentialID, nodeID); err != nil {
		return fmt.Errorf("set sticky node: %w", err)
	}
	return nil
}

func (repo *CredentialRepository) ListStickyStates(ctx context.Context) ([]StickyState, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT credential_id, node_id, updated_at
FROM credential_sticky_states
ORDER BY credential_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list sticky states: %w", err)
	}
	defer rows.Close()

	var states []StickyState
	for rows.Next() {
		var state StickyState
		if err := rows.Scan(&state.CredentialID, &state.NodeID, &state.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sticky state: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sticky states: %w", err)
	}
	return states, nil
}

func (repo *CredentialRepository) ListBindings(ctx context.Context, credentialID int64) ([]model.CredentialBinding, error) {
	rows, err := repo.db.QueryContext(ctx, `
SELECT id, credential_id, target_type, target_id, created_at
FROM credential_bindings
WHERE credential_id = ?
ORDER BY id ASC
`, credentialID)
	if err != nil {
		return nil, fmt.Errorf("list credential bindings: %w", err)
	}
	defer rows.Close()

	var bindings []model.CredentialBinding
	for rows.Next() {
		binding, err := scanCredentialBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, *binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credential bindings: %w", err)
	}
	return bindings, nil
}

func replaceCredentialBindings(ctx context.Context, tx *sql.Tx, credentialID int64, bindings []CredentialBindingTarget) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM credential_bindings WHERE credential_id = ?", credentialID); err != nil {
		return fmt.Errorf("delete credential bindings: %w", err)
	}
	if len(bindings) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO credential_bindings (credential_id, target_type, target_id)
VALUES (?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("prepare credential bindings: %w", err)
	}
	defer stmt.Close()

	for _, binding := range bindings {
		if binding.TargetType != "group" && binding.TargetType != "node" {
			return fmt.Errorf("invalid binding target type %q", binding.TargetType)
		}
		if binding.TargetID <= 0 {
			return errors.New("binding target id must be positive")
		}
		if _, err := stmt.ExecContext(ctx, credentialID, binding.TargetType, binding.TargetID); err != nil {
			return fmt.Errorf("insert credential binding: %w", err)
		}
	}
	return nil
}

func scanCredential(row scanner) (*model.Credential, error) {
	var credential model.Credential
	var enabled int
	var bindMode, selectionPolicy string
	if err := row.Scan(&credential.ID, &credential.Username, &credential.PasswordHash, &enabled,
		&bindMode, &selectionPolicy, &credential.Remark, &credential.CreatedAt, &credential.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan credential: %w", err)
	}
	credential.Enabled = intToBool(enabled)
	credential.BindMode = model.CredentialBindMode(bindMode)
	credential.SelectionPolicy = model.SelectionPolicy(selectionPolicy)
	return &credential, nil
}

func scanCredentialBinding(row scanner) (*model.CredentialBinding, error) {
	var binding model.CredentialBinding
	if err := row.Scan(&binding.ID, &binding.CredentialID, &binding.TargetType, &binding.TargetID, &binding.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan credential binding: %w", err)
	}
	return &binding, nil
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
