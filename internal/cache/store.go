package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/model"
)

var (
	ErrCredentialNotFound = errors.New("credential not found in cache")
	ErrNoCandidateNodes   = errors.New("no candidate proxy nodes")
)

type CredentialSnapshot struct {
	ID              int64
	Username        string
	PasswordHash    string
	Enabled         bool
	BindMode        model.CredentialBindMode
	SelectionPolicy model.SelectionPolicy
	Bindings        []BindingSnapshot
}

type BindingSnapshot struct {
	TargetType string
	TargetID   int64
}

type NodeSnapshot struct {
	ID             int64
	SubscriptionID int64
	Name           string
	Protocol       string
	Server         string
	Port           int
	RawConfigJSON  string
	GroupIDs       []int64
}

type Store struct {
	mu                    sync.RWMutex
	credentialsByUsername map[string]CredentialSnapshot
	nodesByID             map[int64]NodeSnapshot
	groupNodeIDs          map[int64][]int64
	allNodeIDs            []int64
	random                *rand.Rand
}

func NewStore() *Store {
	return &Store{
		credentialsByUsername: make(map[string]CredentialSnapshot),
		nodesByID:             make(map[int64]NodeSnapshot),
		groupNodeIDs:          make(map[int64][]int64),
		random:                rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (store *Store) Load(ctx context.Context, db *sql.DB) error {
	loaded, err := loadSnapshot(ctx, db)
	if err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	store.credentialsByUsername = loaded.credentialsByUsername
	store.nodesByID = loaded.nodesByID
	store.groupNodeIDs = loaded.groupNodeIDs
	store.allNodeIDs = loaded.allNodeIDs
	if store.random == nil {
		store.random = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return nil
}

func (store *Store) Credential(username string) (CredentialSnapshot, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	credential, ok := store.credentialsByUsername[username]
	return credential, ok
}

func (store *Store) SelectNode(username string) (NodeSnapshot, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	credential, ok := store.credentialsByUsername[username]
	if !ok || !credential.Enabled {
		return NodeSnapshot{}, ErrCredentialNotFound
	}

	candidateIDs := store.candidateIDsLocked(credential)
	if len(candidateIDs) == 0 {
		return NodeSnapshot{}, ErrNoCandidateNodes
	}

	selectedID := candidateIDs[0]
	if credential.SelectionPolicy == model.SelectionPolicyRandom && len(candidateIDs) > 1 {
		selectedID = candidateIDs[store.random.Intn(len(candidateIDs))]
	}
	node, ok := store.nodesByID[selectedID]
	if !ok {
		return NodeSnapshot{}, ErrNoCandidateNodes
	}
	return node, nil
}

func (store *Store) candidateIDsLocked(credential CredentialSnapshot) []int64 {
	switch credential.BindMode {
	case model.CredentialBindModeAll:
		return append([]int64(nil), store.allNodeIDs...)
	case model.CredentialBindModeGroup:
		seen := make(map[int64]struct{})
		var candidates []int64
		for _, binding := range credential.Bindings {
			if binding.TargetType != "group" {
				continue
			}
			for _, nodeID := range store.groupNodeIDs[binding.TargetID] {
				if _, ok := seen[nodeID]; ok {
					continue
				}
				seen[nodeID] = struct{}{}
				candidates = append(candidates, nodeID)
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
		return candidates
	case model.CredentialBindModeNode:
		var candidates []int64
		for _, binding := range credential.Bindings {
			if binding.TargetType != "node" {
				continue
			}
			if _, ok := store.nodesByID[binding.TargetID]; ok {
				candidates = append(candidates, binding.TargetID)
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
		return candidates
	default:
		return nil
	}
}

func loadSnapshot(ctx context.Context, db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("database handle is nil")
	}
	store := NewStore()

	if err := loadCredentials(ctx, db, store); err != nil {
		return nil, err
	}
	if err := loadNodes(ctx, db, store); err != nil {
		return nil, err
	}
	if err := loadNodeGroups(ctx, db, store); err != nil {
		return nil, err
	}
	return store, nil
}

func loadCredentials(ctx context.Context, db *sql.DB, store *Store) error {
	rows, err := db.QueryContext(ctx, `
SELECT id, username, password_hash, enabled, bind_mode, selection_policy
FROM credentials
WHERE enabled = 1
ORDER BY id ASC
`)
	if err != nil {
		return fmt.Errorf("load credentials cache: %w", err)
	}
	defer rows.Close()

	byID := make(map[int64]string)
	for rows.Next() {
		var credential CredentialSnapshot
		var enabled int
		var bindMode, selectionPolicy string
		if err := rows.Scan(&credential.ID, &credential.Username, &credential.PasswordHash, &enabled, &bindMode, &selectionPolicy); err != nil {
			return fmt.Errorf("scan credential cache: %w", err)
		}
		credential.Enabled = enabled != 0
		credential.BindMode = model.CredentialBindMode(bindMode)
		credential.SelectionPolicy = model.SelectionPolicy(selectionPolicy)
		store.credentialsByUsername[credential.Username] = credential
		byID[credential.ID] = credential.Username
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate credential cache: %w", err)
	}

	bindingRows, err := db.QueryContext(ctx, `
SELECT credential_id, target_type, target_id
FROM credential_bindings
ORDER BY id ASC
`)
	if err != nil {
		return fmt.Errorf("load credential bindings cache: %w", err)
	}
	defer bindingRows.Close()
	for bindingRows.Next() {
		var credentialID int64
		var binding BindingSnapshot
		if err := bindingRows.Scan(&credentialID, &binding.TargetType, &binding.TargetID); err != nil {
			return fmt.Errorf("scan credential binding cache: %w", err)
		}
		username, ok := byID[credentialID]
		if !ok {
			continue
		}
		credential := store.credentialsByUsername[username]
		credential.Bindings = append(credential.Bindings, binding)
		store.credentialsByUsername[username] = credential
	}
	if err := bindingRows.Err(); err != nil {
		return fmt.Errorf("iterate credential binding cache: %w", err)
	}
	return nil
}

func loadNodes(ctx context.Context, db *sql.DB, store *Store) error {
	rows, err := db.QueryContext(ctx, `
SELECT id, subscription_id, name, protocol, server, port, raw_config_json
FROM proxy_nodes
WHERE enabled = 1 AND adapter_status = 'supported' AND alive_status != 'dead'
ORDER BY id ASC
`)
	if err != nil {
		return fmt.Errorf("load nodes cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var node NodeSnapshot
		if err := rows.Scan(&node.ID, &node.SubscriptionID, &node.Name, &node.Protocol, &node.Server, &node.Port, &node.RawConfigJSON); err != nil {
			return fmt.Errorf("scan node cache: %w", err)
		}
		store.nodesByID[node.ID] = node
		store.allNodeIDs = append(store.allNodeIDs, node.ID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate node cache: %w", err)
	}
	return nil
}

func loadNodeGroups(ctx context.Context, db *sql.DB, store *Store) error {
	rows, err := db.QueryContext(ctx, `
SELECT node_id, group_id
FROM proxy_node_groups
ORDER BY group_id ASC, node_id ASC
`)
	if err != nil {
		return fmt.Errorf("load node groups cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var nodeID, groupID int64
		if err := rows.Scan(&nodeID, &groupID); err != nil {
			return fmt.Errorf("scan node group cache: %w", err)
		}
		node, ok := store.nodesByID[nodeID]
		if !ok {
			continue
		}
		node.GroupIDs = append(node.GroupIDs, groupID)
		store.nodesByID[nodeID] = node
		store.groupNodeIDs[groupID] = append(store.groupNodeIDs[groupID], nodeID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate node group cache: %w", err)
	}
	return nil
}
