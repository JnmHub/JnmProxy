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
	ID                  int64
	SubscriptionID      int64
	Name                string
	Protocol            string
	Server              string
	Port                int
	RawConfigJSON       string
	SingBoxOutboundJSON string
	SingBoxStatus       model.SingBoxStatus
	SingBoxError        string
	SingBoxVersion      string
	UDPSupported        bool
	TransportType       string
	GroupIDs            []int64
}

type Selection struct {
	Credential CredentialSnapshot
	Node       NodeSnapshot
	GroupID    int64
}

type RuntimeOptions struct {
	FailureThreshold     int
	CircuitBreakDuration time.Duration
}

type Store struct {
	mu                    sync.RWMutex
	credentialsByUsername map[string]CredentialSnapshot
	nodesByID             map[int64]NodeSnapshot
	groupNodeIDs          map[int64][]int64
	allNodeIDs            []int64
	random                *rand.Rand
	selectionBags         map[int64]*selectionBag
	nodeFailures          map[int64]nodeFailureState
	failureThreshold      int
	circuitBreakDuration  time.Duration
}

type selectionBag struct {
	candidates   []int64
	order        []int64
	index        int
	lastSelected int64
}

type nodeFailureState struct {
	Count        int
	CircuitUntil time.Time
	LastFailure  string
	LastFailedAt time.Time
}

func NewStore() *Store {
	return &Store{
		credentialsByUsername: make(map[string]CredentialSnapshot),
		nodesByID:             make(map[int64]NodeSnapshot),
		groupNodeIDs:          make(map[int64][]int64),
		random:                rand.New(rand.NewSource(time.Now().UnixNano())),
		selectionBags:         make(map[int64]*selectionBag),
		nodeFailures:          make(map[int64]nodeFailureState),
		failureThreshold:      3,
		circuitBreakDuration:  time.Minute,
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
	store.selectionBags = make(map[int64]*selectionBag)
	if store.nodeFailures == nil {
		store.nodeFailures = make(map[int64]nodeFailureState)
	}
	if store.failureThreshold <= 0 {
		store.failureThreshold = 3
	}
	if store.circuitBreakDuration <= 0 {
		store.circuitBreakDuration = time.Minute
	}
	return nil
}

func (store *Store) Credential(username string) (CredentialSnapshot, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	credential, ok := store.credentialsByUsername[username]
	return credential, ok
}

func (store *Store) ConfigureRuntime(options RuntimeOptions) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if options.FailureThreshold > 0 {
		store.failureThreshold = options.FailureThreshold
	}
	if options.CircuitBreakDuration > 0 {
		store.circuitBreakDuration = options.CircuitBreakDuration
	}
}

func (store *Store) SelectNode(username string) (NodeSnapshot, error) {
	selection, err := store.Select(username)
	if err != nil {
		return NodeSnapshot{}, err
	}
	return selection.Node, nil
}

func (store *Store) Select(username string) (Selection, error) {
	return store.SelectExcluding(username, nil)
}

func (store *Store) SelectExcluding(username string, excludedNodeIDs map[int64]struct{}) (Selection, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	credential, ok := store.credentialsByUsername[username]
	if !ok || !credential.Enabled {
		return Selection{}, ErrCredentialNotFound
	}

	candidateIDs := store.candidateIDsLocked(credential)
	if len(excludedNodeIDs) > 0 {
		filtered := make([]int64, 0, len(candidateIDs))
		for _, nodeID := range candidateIDs {
			if _, excluded := excludedNodeIDs[nodeID]; excluded {
				continue
			}
			filtered = append(filtered, nodeID)
		}
		candidateIDs = filtered
	}
	if len(candidateIDs) == 0 {
		return Selection{}, ErrNoCandidateNodes
	}

	selectedID := candidateIDs[0]
	if credential.SelectionPolicy == model.SelectionPolicyRandom && len(candidateIDs) > 1 {
		selectedID = store.selectRandomNodeLocked(credential, candidateIDs)
	}
	node, ok := store.nodesByID[selectedID]
	if !ok {
		return Selection{}, ErrNoCandidateNodes
	}
	return Selection{
		Credential: credential,
		Node:       node,
		GroupID:    store.selectedGroupIDLocked(credential, selectedID),
	}, nil
}

func (store *Store) ReportNodeFailure(nodeID int64, reasons ...string) {
	store.mu.Lock()
	defer store.mu.Unlock()

	state := store.nodeFailures[nodeID]
	state.Count++
	if len(reasons) > 0 {
		state.LastFailure = reasons[0]
		state.LastFailedAt = time.Now()
	}
	if state.Count >= store.failureThreshold {
		state.CircuitUntil = time.Now().Add(store.circuitBreakDuration)
	}
	store.nodeFailures[nodeID] = state
}

func (store *Store) ReportNodeSuccess(nodeID int64) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.nodeFailures, nodeID)
}

func (store *Store) selectRandomNodeLocked(credential CredentialSnapshot, candidateIDs []int64) int64 {
	if len(candidateIDs) == 1 {
		return candidateIDs[0]
	}
	if store.selectionBags == nil {
		store.selectionBags = make(map[int64]*selectionBag)
	}
	bag := store.selectionBags[credential.ID]
	if bag == nil {
		bag = &selectionBag{}
		store.selectionBags[credential.ID] = bag
	}
	if !sameNodeIDs(bag.candidates, candidateIDs) {
		bag.reset(candidateIDs, store.random)
	}
	return bag.next(store.random)
}

func (bag *selectionBag) reset(candidateIDs []int64, random *rand.Rand) {
	bag.candidates = append(bag.candidates[:0], candidateIDs...)
	bag.order = append(bag.order[:0], candidateIDs...)
	bag.index = 0
	bag.shuffle(random)
}

func (bag *selectionBag) next(random *rand.Rand) int64 {
	if bag.index >= len(bag.order) {
		bag.order = append(bag.order[:0], bag.candidates...)
		bag.index = 0
		bag.shuffle(random)
	}
	selectedID := bag.order[bag.index]
	bag.index++
	bag.lastSelected = selectedID
	return selectedID
}

func (bag *selectionBag) shuffle(random *rand.Rand) {
	random.Shuffle(len(bag.order), func(i int, j int) {
		bag.order[i], bag.order[j] = bag.order[j], bag.order[i]
	})
	if len(bag.order) <= 1 || bag.lastSelected == 0 || bag.order[0] != bag.lastSelected {
		return
	}
	swapIndex := 1 + random.Intn(len(bag.order)-1)
	bag.order[0], bag.order[swapIndex] = bag.order[swapIndex], bag.order[0]
}

func sameNodeIDs(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (store *Store) candidateIDsLocked(credential CredentialSnapshot) []int64 {
	switch credential.BindMode {
	case model.CredentialBindModeAll:
		return store.filterAvailableLocked(store.allNodeIDs)
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
		candidates = store.filterAvailableLocked(candidates)
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
		candidates = store.filterAvailableLocked(candidates)
		sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
		return candidates
	default:
		return nil
	}
}

func (store *Store) filterAvailableLocked(nodeIDs []int64) []int64 {
	now := time.Now()
	candidates := make([]int64, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		state, failed := store.nodeFailures[nodeID]
		if failed && !state.CircuitUntil.IsZero() {
			if now.Before(state.CircuitUntil) {
				continue
			}
			delete(store.nodeFailures, nodeID)
		}
		candidates = append(candidates, nodeID)
	}
	return candidates
}

func (store *Store) selectedGroupIDLocked(credential CredentialSnapshot, nodeID int64) int64 {
	if credential.BindMode != model.CredentialBindModeGroup {
		return 0
	}
	for _, binding := range credential.Bindings {
		if binding.TargetType != "group" {
			continue
		}
		for _, candidateID := range store.groupNodeIDs[binding.TargetID] {
			if candidateID == nodeID {
				return binding.TargetID
			}
		}
	}
	return 0
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
SELECT id, subscription_id, name, protocol, server, port, raw_config_json,
       sing_box_outbound_json, sing_box_status, sing_box_error, sing_box_version,
       udp_supported, transport_type
FROM proxy_nodes
WHERE enabled = 1 AND (adapter_status = 'supported' OR sing_box_status = 'supported') AND alive_status != 'dead'
ORDER BY id ASC
`)
	if err != nil {
		return fmt.Errorf("load nodes cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var node NodeSnapshot
		var udpSupported int
		if err := rows.Scan(&node.ID, &node.SubscriptionID, &node.Name, &node.Protocol, &node.Server, &node.Port,
			&node.RawConfigJSON, &node.SingBoxOutboundJSON, &node.SingBoxStatus, &node.SingBoxError,
			&node.SingBoxVersion, &udpSupported, &node.TransportType); err != nil {
			return fmt.Errorf("scan node cache: %w", err)
		}
		node.UDPSupported = udpSupported != 0
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
