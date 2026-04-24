package grouping

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/repository"
)

func TestManualAndKeywordGrouping(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	subRepo := repository.NewSubscriptionRepository(store)
	subscription, err := subRepo.Create(ctx, repository.CreateSubscriptionParams{
		Name:    "测试订阅",
		URL:     "https://example.com/sub",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if err := subRepo.UpsertNodes(ctx, []repository.UpsertProxyNodeParams{
		{
			SubscriptionID:      subscription.ID,
			SubscriptionNodeKey: "node-hk",
			Name:                "香港 HK 01",
			Protocol:            "http",
			Server:              "hk.example.com",
			Port:                8080,
			RawConfigJSON:       "{}",
			AdapterStatus:       "supported",
		},
		{
			SubscriptionID:      subscription.ID,
			SubscriptionNodeKey: "node-jp",
			Name:                "日本 JP 01",
			Protocol:            "http",
			Server:              "jp.example.com",
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
	groupRepo := repository.NewGroupRepository(store)
	manualGroup, err := groupRepo.CreateGroup(ctx, repository.CreateGroupParams{Name: "手动分组"})
	if err != nil {
		t.Fatalf("create manual group: %v", err)
	}
	if err := groupRepo.AddNodesToGroup(ctx, manualGroup.ID, []int64{nodes[0].ID, nodes[1].ID}); err != nil {
		t.Fatalf("manual add nodes: %v", err)
	}

	rule, err := groupRepo.CreateKeywordRule(ctx, repository.CreateKeywordParams{
		Name:     "地区规则",
		Keywords: "香港|HK|日本",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create keyword rule: %v", err)
	}

	service := NewService(groupRepo)
	result, err := service.ApplyKeywordGroups(ctx, ApplyKeywordParams{RuleIDs: []int64{rule.ID}})
	if err != nil {
		t.Fatalf("apply keywords: %v", err)
	}
	if result.RulesScanned != 1 || result.NodesScanned != 2 || result.GroupsTouched != 3 {
		t.Fatalf("unexpected apply result: %#v", result)
	}

	hkGroups, err := groupRepo.ListGroupsByNode(ctx, nodes[0].ID)
	if err != nil {
		t.Fatalf("list hk groups: %v", err)
	}
	if !hasGroup(hkGroups, "手动分组") || !hasGroup(hkGroups, "香港") || !hasGroup(hkGroups, "HK") {
		t.Fatalf("hk node missing groups: %#v", hkGroups)
	}
	jpGroups, err := groupRepo.ListGroupsByNode(ctx, nodes[1].ID)
	if err != nil {
		t.Fatalf("list jp groups: %v", err)
	}
	if !hasGroup(jpGroups, "手动分组") || !hasGroup(jpGroups, "日本") || hasGroup(jpGroups, "HK") {
		t.Fatalf("jp node groups mismatch: %#v", jpGroups)
	}
}

func TestApplyAllKeywordRulesSkipsDisabled(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	subRepo := repository.NewSubscriptionRepository(store)
	subscription, err := subRepo.Create(ctx, repository.CreateSubscriptionParams{Name: "sub", URL: "https://example.com", Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if err := subRepo.UpsertNodes(ctx, []repository.UpsertProxyNodeParams{
		{
			SubscriptionID:      subscription.ID,
			SubscriptionNodeKey: "node-us",
			Name:                "美国 US 01",
			Protocol:            "http",
			Server:              "us.example.com",
			Port:                8080,
			RawConfigJSON:       "{}",
			AdapterStatus:       "supported",
		},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	groupRepo := repository.NewGroupRepository(store)
	if _, err := groupRepo.CreateKeywordRule(ctx, repository.CreateKeywordParams{Name: "enabled", Keywords: "美国", Enabled: true}); err != nil {
		t.Fatalf("create enabled rule: %v", err)
	}
	if _, err := groupRepo.CreateKeywordRule(ctx, repository.CreateKeywordParams{Name: "disabled", Keywords: "US", Enabled: false}); err != nil {
		t.Fatalf("create disabled rule: %v", err)
	}

	result, err := NewService(groupRepo).ApplyKeywordGroups(ctx, ApplyKeywordParams{All: true})
	if err != nil {
		t.Fatalf("apply all: %v", err)
	}
	if result.RulesScanned != 1 || result.GroupsTouched != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	groups, err := groupRepo.ListGroups(ctx)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "美国" || !groups[0].AutoCreated {
		t.Fatalf("unexpected groups: %#v", groups)
	}
}

func hasGroup(groups []model.ProxyGroup, name string) bool {
	for _, group := range groups {
		if group.Name == name {
			return true
		}
	}
	return false
}
