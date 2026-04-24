package cache

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jnmproxy/jnmproxy/internal/auth"
	"github.com/jnmproxy/jnmproxy/internal/db"
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
	if allNode.ID != nodes[0].ID {
		t.Fatalf("fixed all credential should select first node, got %d", allNode.ID)
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
	if allNodeAfterSuccess.ID != nodes[0].ID {
		t.Fatalf("success should reset circuit breaker, got %d", allNodeAfterSuccess.ID)
	}
}
