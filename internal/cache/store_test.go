package cache

import (
	"context"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/auth"
	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/repository"
)

func TestStoreSelectsNodesByCredentialBinding(t *testing.T) {
	ctx := context.Background()
	storeDB, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer storeDB.Close()
	if err := db.Migrate(ctx, storeDB); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	subRepo := repository.NewSubscriptionRepository(storeDB)
	subscription, err := subRepo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: "https://example.com", Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if err := subRepo.UpsertNodes(ctx, []repository.UpsertProxyNodeParams{
		{
			SubscriptionID:      subscription.ID,
			SubscriptionNodeKey: "node-1",
			Name:                "node 1",
			Protocol:            "http",
			Server:              "one.example.com",
			Port:                8080,
			RawConfigJSON:       "{}",
			AdapterStatus:       "supported",
		},
		{
			SubscriptionID:      subscription.ID,
			SubscriptionNodeKey: "node-2",
			Name:                "node 2",
			Protocol:            "http",
			Server:              "two.example.com",
			Port:                8080,
			RawConfigJSON:       "{}",
			AdapterStatus:       "supported",
		},
	}); err != nil {
		t.Fatalf("upsert nodes: %v", err)
	}
	nodes, err := subRepo.ListNodesBySubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}

	groupRepo := repository.NewGroupRepository(storeDB)
	group, err := groupRepo.CreateGroup(ctx, repository.CreateGroupParams{Name: "group-a"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := groupRepo.AddNodesToGroup(ctx, group.ID, []int64{nodes[1].ID}); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	authService := auth.NewService(repository.NewCredentialRepository(storeDB))
	if _, err := authService.CreateCredential(ctx, auth.CreateCredentialInput{
		Username:        "all-user",
		Password:        "pass",
		Enabled:         true,
		BindMode:        "all",
		SelectionPolicy: "fixed",
	}); err != nil {
		t.Fatalf("create all credential: %v", err)
	}
	if _, err := authService.CreateCredential(ctx, auth.CreateCredentialInput{
		Username:        "group-user",
		Password:        "pass",
		Enabled:         true,
		BindMode:        "group",
		SelectionPolicy: "fixed",
		Bindings: []repository.CredentialBindingTarget{
			{TargetType: "group", TargetID: group.ID},
		},
	}); err != nil {
		t.Fatalf("create group credential: %v", err)
	}
	if _, err := authService.CreateCredential(ctx, auth.CreateCredentialInput{
		Username:        "node-user",
		Password:        "pass",
		Enabled:         true,
		BindMode:        "node",
		SelectionPolicy: "fixed",
		Bindings: []repository.CredentialBindingTarget{
			{TargetType: "node", TargetID: nodes[0].ID},
		},
	}); err != nil {
		t.Fatalf("create node credential: %v", err)
	}

	cacheStore := NewStore()
	if err := cacheStore.Load(ctx, storeDB); err != nil {
		t.Fatalf("load cache: %v", err)
	}

	allNode, err := cacheStore.SelectNode("all-user")
	if err != nil {
		t.Fatalf("select all node: %v", err)
	}
	if allNode.ID != nodes[0].ID && allNode.ID != nodes[1].ID {
		t.Fatalf("all credential should select one available node, got %d", allNode.ID)
	}

	groupNode, err := cacheStore.SelectNode("group-user")
	if err != nil {
		t.Fatalf("select group node: %v", err)
	}
	if groupNode.ID != nodes[1].ID {
		t.Fatalf("group credential should select grouped node, got %d", groupNode.ID)
	}

	node, err := cacheStore.SelectNode("node-user")
	if err != nil {
		t.Fatalf("select fixed node: %v", err)
	}
	if node.ID != nodes[0].ID {
		t.Fatalf("node credential should select bound node, got %d", node.ID)
	}

	cacheStore.failureThreshold = 1
	cacheStore.ReportNodeFailure(nodes[0].ID)
	allNodeAfterFailure, err := cacheStore.SelectNode("all-user")
	if err != nil {
		t.Fatalf("select all after circuit break: %v", err)
	}
	if allNodeAfterFailure.ID != nodes[1].ID {
		t.Fatalf("circuit breaker should skip failed first node, got %d", allNodeAfterFailure.ID)
	}
	cacheStore.ReportNodeSuccess(nodes[0].ID)
	allNodeAfterSuccess, err := cacheStore.SelectNode("all-user")
	if err != nil {
		t.Fatalf("select all after success: %v", err)
	}
	if allNodeAfterSuccess.ID != nodes[0].ID && allNodeAfterSuccess.ID != nodes[1].ID {
		t.Fatalf("success should restore failed node to candidate set, got %d", allNodeAfterSuccess.ID)
	}
}

func TestSelectExcludingSkipsAlreadyTriedNodes(t *testing.T) {
	store := NewStore()
	store.credentialsByUsername["fixed-user"] = CredentialSnapshot{
		ID:              1,
		Username:        "fixed-user",
		Enabled:         true,
		BindMode:        model.CredentialBindModeNode,
		SelectionPolicy: model.SelectionPolicyFixed,
		Bindings: []BindingSnapshot{
			{TargetType: "node", TargetID: 1},
			{TargetType: "node", TargetID: 2},
			{TargetType: "node", TargetID: 3},
		},
	}
	for _, nodeID := range []int64{1, 2, 3} {
		store.nodesByID[nodeID] = NodeSnapshot{ID: nodeID}
	}

	first, err := store.SelectExcluding("fixed-user", nil)
	if err != nil {
		t.Fatalf("select first: %v", err)
	}
	if first.Node.ID != 1 {
		t.Fatalf("expected first node 1, got %d", first.Node.ID)
	}

	second, err := store.SelectExcluding("fixed-user", map[int64]struct{}{1: {}})
	if err != nil {
		t.Fatalf("select second: %v", err)
	}
	if second.Node.ID != 2 {
		t.Fatalf("expected excluded node 1 to be skipped, got %d", second.Node.ID)
	}

	_, err = store.SelectExcluding("fixed-user", map[int64]struct{}{1: {}, 2: {}, 3: {}})
	if err != ErrNoCandidateNodes {
		t.Fatalf("expected no candidates after excluding all nodes, got %v", err)
	}
}

func TestShuffleBagSelectsEachCandidateOncePerRound(t *testing.T) {
	store := newShuffleTestStore("random-user", []int64{1, 2, 3, 4, 5})

	firstRound := selectNodeIDs(t, store, "random-user", 5)
	assertUniqueSet(t, firstRound, []int64{1, 2, 3, 4, 5})

	secondRound := selectNodeIDs(t, store, "random-user", 5)
	assertUniqueSet(t, secondRound, []int64{1, 2, 3, 4, 5})

	if firstRound[len(firstRound)-1] == secondRound[0] {
		t.Fatalf("shuffle bag should avoid immediate cross-round repeat: first=%v second=%v", firstRound, secondRound)
	}
}

func TestShuffleBagRebuildsWhenCandidatesChange(t *testing.T) {
	store := newShuffleTestStore("random-user", []int64{1, 2, 3})
	_ = selectNodeIDs(t, store, "random-user", 1)

	store.failureThreshold = 1
	store.circuitBreakDuration = time.Hour
	store.ReportNodeFailure(3)

	afterFailure := selectNodeIDs(t, store, "random-user", 2)
	assertUniqueSet(t, afterFailure, []int64{1, 2})

	store.ReportNodeSuccess(3)
	afterSuccess := selectNodeIDs(t, store, "random-user", 3)
	assertUniqueSet(t, afterSuccess, []int64{1, 2, 3})
}

func TestShuffleBagDeduplicatesGroupedCandidates(t *testing.T) {
	store := NewStore()
	store.random = rand.New(rand.NewSource(3))
	store.credentialsByUsername["group-user"] = CredentialSnapshot{
		ID:              1,
		Username:        "group-user",
		Enabled:         true,
		BindMode:        model.CredentialBindModeGroup,
		SelectionPolicy: model.SelectionPolicyRandom,
		Bindings: []BindingSnapshot{
			{TargetType: "group", TargetID: 10},
			{TargetType: "group", TargetID: 20},
		},
	}
	for _, nodeID := range []int64{1, 2, 3} {
		store.nodesByID[nodeID] = NodeSnapshot{ID: nodeID}
	}
	store.groupNodeIDs[10] = []int64{1, 2}
	store.groupNodeIDs[20] = []int64{2, 3}

	selected := selectNodeIDs(t, store, "group-user", 3)
	assertUniqueSet(t, selected, []int64{1, 2, 3})
}

func TestFixedNodeSelectionDoesNotUseShuffleBag(t *testing.T) {
	store := NewStore()
	store.random = rand.New(rand.NewSource(4))
	store.credentialsByUsername["node-user"] = CredentialSnapshot{
		ID:              1,
		Username:        "node-user",
		Enabled:         true,
		BindMode:        model.CredentialBindModeNode,
		SelectionPolicy: model.SelectionPolicyFixed,
		Bindings:        []BindingSnapshot{{TargetType: "node", TargetID: 2}},
	}
	for _, nodeID := range []int64{1, 2, 3} {
		store.nodesByID[nodeID] = NodeSnapshot{ID: nodeID}
	}

	selected := selectNodeIDs(t, store, "node-user", 3)
	for _, nodeID := range selected {
		if nodeID != 2 {
			t.Fatalf("fixed node credential should always select node 2, got %v", selected)
		}
	}
	if len(store.selectionBags) != 0 {
		t.Fatalf("fixed node selection should not create shuffle bags: %#v", store.selectionBags)
	}
}

func TestStoreLoadClearsShuffleBags(t *testing.T) {
	ctx := context.Background()
	storeDB, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer storeDB.Close()
	if err := db.Migrate(ctx, storeDB); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	store := NewStore()
	store.selectionBags[1] = &selectionBag{candidates: []int64{1}, order: []int64{1}, index: 1, lastSelected: 1}
	if err := store.Load(ctx, storeDB); err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if len(store.selectionBags) != 0 {
		t.Fatalf("load should clear shuffle bags, got %#v", store.selectionBags)
	}
}

func newShuffleTestStore(username string, nodeIDs []int64) *Store {
	store := NewStore()
	store.random = rand.New(rand.NewSource(2))
	store.credentialsByUsername[username] = CredentialSnapshot{
		ID:              1,
		Username:        username,
		Enabled:         true,
		BindMode:        model.CredentialBindModeAll,
		SelectionPolicy: model.SelectionPolicyRandom,
	}
	for _, nodeID := range nodeIDs {
		store.nodesByID[nodeID] = NodeSnapshot{ID: nodeID}
		store.allNodeIDs = append(store.allNodeIDs, nodeID)
	}
	return store
}

func selectNodeIDs(t *testing.T, store *Store, username string, count int) []int64 {
	t.Helper()
	selected := make([]int64, 0, count)
	for range count {
		node, err := store.SelectNode(username)
		if err != nil {
			t.Fatalf("select node: %v", err)
		}
		selected = append(selected, node.ID)
	}
	return selected
}

func assertUniqueSet(t *testing.T, actual []int64, expected []int64) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("expected %d selections, got %d: %v", len(expected), len(actual), actual)
	}
	seen := make(map[int64]struct{}, len(actual))
	for _, nodeID := range actual {
		if _, ok := seen[nodeID]; ok {
			t.Fatalf("selection should not repeat within a round: %v", actual)
		}
		seen[nodeID] = struct{}{}
	}
	for _, nodeID := range expected {
		if _, ok := seen[nodeID]; !ok {
			t.Fatalf("selection missing node %d: actual=%v expected=%v", nodeID, actual, expected)
		}
	}
}
