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
	return service.repo.Create(ctx, repository.CreateCredentialParams{
		Username:        input.Username,
		PasswordHash:    hash,
		Enabled:         input.Enabled,
		BindMode:        input.BindMode,
		SelectionPolicy: input.SelectionPolicy,
		Remark:          input.Remark,
		Bindings:        input.Bindings,
	})
}

func (service *Service) UpdateCredential(ctx context.Context, id int64, input UpdateCredentialInput) (*model.Credential, error) {
	return service.repo.Update(ctx, id, repository.UpdateCredentialParams{
		Enabled:         input.Enabled,
		BindMode:        input.BindMode,
		SelectionPolicy: input.SelectionPolicy,
		Remark:          input.Remark,
		Bindings:        input.Bindings,
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
