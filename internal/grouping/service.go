package grouping

import (
	"context"
	"strings"

	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/repository"
)

type Service struct {
	repo *repository.GroupRepository
}

type ApplyKeywordParams struct {
	RuleIDs []int64
	All     bool
}

type ApplyKeywordResult struct {
	RulesScanned     int `json:"rules_scanned"`
	NodesScanned     int `json:"nodes_scanned"`
	GroupsTouched    int `json:"groups_touched"`
	RelationsTouched int `json:"relations_touched"`
}

func NewService(repo *repository.GroupRepository) *Service {
	return &Service{repo: repo}
}

func (service *Service) ApplyKeywordGroups(ctx context.Context, params ApplyKeywordParams) (*ApplyKeywordResult, error) {
	rules, err := service.rules(ctx, params)
	if err != nil {
		return nil, err
	}
	nodes, err := service.repo.ListNodeNames(ctx)
	if err != nil {
		return nil, err
	}

	result := &ApplyKeywordResult{
		RulesScanned: len(rules),
		NodesScanned: len(nodes),
	}
	touchedGroups := make(map[int64]struct{})

	for _, rule := range rules {
		keywords := splitKeywords(rule.Keywords)
		if len(keywords) == 0 {
			continue
		}
		for _, node := range nodes {
			for _, keyword := range keywords {
				if !matches(node.Name, keyword, rule.CaseSensitive) {
					continue
				}
				group, err := service.repo.EnsureGroup(ctx, keyword, true)
				if err != nil {
					return nil, err
				}
				touchedGroups[group.ID] = struct{}{}
				if err := service.repo.AddNodesToGroup(ctx, group.ID, []int64{node.ID}); err != nil {
					return nil, err
				}
				result.RelationsTouched++
			}
		}
	}

	result.GroupsTouched = len(touchedGroups)
	return result, nil
}

func (service *Service) rules(ctx context.Context, params ApplyKeywordParams) ([]model.GroupKeyword, error) {
	if params.All {
		return service.repo.ListKeywordRules(ctx, true)
	}
	return service.repo.ListKeywordRulesByIDs(ctx, params.RuleIDs)
}

func splitKeywords(value string) []string {
	parts := strings.Split(value, "|")
	keywords := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		keyword := strings.TrimSpace(part)
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	return keywords
}

func matches(name string, keyword string, caseSensitive bool) bool {
	if !caseSensitive {
		name = strings.ToLower(name)
		keyword = strings.ToLower(keyword)
	}
	return strings.Contains(name, keyword)
}
