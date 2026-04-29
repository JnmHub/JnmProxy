package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jnmproxy/jnmproxy/internal/model"
	"github.com/jnmproxy/jnmproxy/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredential = errors.New("invalid credential")

type Service struct {
	repo *repository.CredentialRepository
}

type CreateCredentialInput struct {
	Username        string
	Password        string
	Enabled         bool
	BindMode        string
	SelectionPolicy string
	Remark          string
	Bindings        []repository.CredentialBindingTarget
}

type UpdateCredentialInput struct {
	Enabled         *bool
	BindMode        *string
	SelectionPolicy *string
	Remark          *string
	Bindings        *[]repository.CredentialBindingTarget
}

func NewService(repo *repository.CredentialRepository) *Service {
	return &Service{repo: repo}
}

func (service *Service) CreateCredential(ctx context.Context, input CreateCredentialInput) (*model.Credential, error) {
	hash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	bindMode, selectionPolicy, bindings, err := normalizeCredentialScope(input.BindMode, input.SelectionPolicy, input.Bindings)
	if err != nil {
		return nil, err
	}
	return service.repo.Create(ctx, repository.CreateCredentialParams{
		Username:        input.Username,
		PasswordHash:    hash,
		Enabled:         input.Enabled,
		BindMode:        bindMode,
		SelectionPolicy: selectionPolicy,
		Remark:          input.Remark,
		Bindings:        bindings,
	})
}

func (service *Service) UpdateCredential(ctx context.Context, id int64, input UpdateCredentialInput) (*model.Credential, error) {
	current, err := service.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	currentBindings, err := service.repo.ListBindings(ctx, id)
	if err != nil {
		return nil, err
	}

	bindMode := string(current.BindMode)
	if input.BindMode != nil {
		bindMode = *input.BindMode
	}
	selectionPolicy := string(current.SelectionPolicy)
	if input.SelectionPolicy != nil {
		selectionPolicy = *input.SelectionPolicy
	}
	bindings := credentialBindingTargets(currentBindings)
	if input.Bindings != nil {
		bindings = *input.Bindings
	}
	bindMode, selectionPolicy, bindings, err = normalizeCredentialScope(bindMode, selectionPolicy, bindings)
	if err != nil {
		return nil, err
	}

	return service.repo.Update(ctx, id, repository.UpdateCredentialParams{
		Enabled:         input.Enabled,
		BindMode:        &bindMode,
		SelectionPolicy: &selectionPolicy,
		Remark:          input.Remark,
		Bindings:        &bindings,
	})
}

func (service *Service) Authenticate(ctx context.Context, username string, password string) (*model.Credential, error) {
	credential, err := service.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredential
	}
	if !credential.Enabled {
		return nil, ErrInvalidCredential
	}
	if !VerifyPassword(credential.PasswordHash, password) {
		return nil, ErrInvalidCredential
	}
	return credential, nil
}

func (service *Service) ResetPassword(ctx context.Context, id int64, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return service.repo.ResetPassword(ctx, id, hash)
}

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func VerifyPassword(hash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func normalizeCredentialScope(bindMode string, selectionPolicy string, bindings []repository.CredentialBindingTarget) (string, string, []repository.CredentialBindingTarget, error) {
	if bindMode == "" {
		bindMode = string(model.CredentialBindModeAll)
	}
	if selectionPolicy == "" {
		selectionPolicy = string(model.SelectionPolicyRandom)
	}

	switch model.CredentialBindMode(bindMode) {
	case model.CredentialBindModeAll:
		return bindMode, normalizeRangeSelectionPolicy(selectionPolicy), nil, nil
	case model.CredentialBindModeGroup:
		groupBindings := filterCredentialBindings(bindings, "group")
		if len(groupBindings) == 0 {
			return "", "", nil, errors.New("group binding requires at least one group")
		}
		return bindMode, normalizeRangeSelectionPolicy(selectionPolicy), groupBindings, nil
	case model.CredentialBindModeNode:
		nodeBindings := filterCredentialBindings(bindings, "node")
		if len(nodeBindings) != 1 {
			return "", "", nil, errors.New("fixed node binding requires exactly one node")
		}
		return bindMode, string(model.SelectionPolicyFixed), nodeBindings, nil
	default:
		return "", "", nil, errors.New("invalid credential bind mode")
	}
}

func normalizeRangeSelectionPolicy(selectionPolicy string) string {
	if model.SelectionPolicy(selectionPolicy) == model.SelectionPolicySticky {
		return string(model.SelectionPolicySticky)
	}
	return string(model.SelectionPolicyRandom)
}

func filterCredentialBindings(bindings []repository.CredentialBindingTarget, targetType string) []repository.CredentialBindingTarget {
	filtered := make([]repository.CredentialBindingTarget, 0, len(bindings))
	seen := make(map[int64]struct{})
	for _, binding := range bindings {
		if binding.TargetType != targetType || binding.TargetID <= 0 {
			continue
		}
		if _, ok := seen[binding.TargetID]; ok {
			continue
		}
		seen[binding.TargetID] = struct{}{}
		filtered = append(filtered, repository.CredentialBindingTarget{TargetType: targetType, TargetID: binding.TargetID})
	}
	return filtered
}

func credentialBindingTargets(bindings []model.CredentialBinding) []repository.CredentialBindingTarget {
	targets := make([]repository.CredentialBindingTarget, 0, len(bindings))
	for _, binding := range bindings {
		targets = append(targets, repository.CredentialBindingTarget{
			TargetType: binding.TargetType,
			TargetID:   binding.TargetID,
		})
	}
	return targets
}
