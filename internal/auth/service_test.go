package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/jnmproxy/jnmproxy/internal/db"
	"github.com/jnmproxy/jnmproxy/internal/repository"
)

func TestCreateAndAuthenticateCredential(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repo := repository.NewCredentialRepository(store)
	service := NewService(repo)

	credential, err := service.CreateCredential(ctx, CreateCredentialInput{
		Username:        "15376259491",
		Password:        "00hhg5210",
		Enabled:         true,
		BindMode:        "all",
		SelectionPolicy: "random",
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	if credential.PasswordHash == "00hhg5210" {
		t.Fatal("password was stored in plaintext")
	}

	authenticated, err := service.Authenticate(ctx, "15376259491", "00hhg5210")
	if err != nil {
		t.Fatalf("authenticate credential: %v", err)
	}
	if authenticated.ID != credential.ID {
		t.Fatalf("unexpected credential id: %d", authenticated.ID)
	}
	if _, err := service.Authenticate(ctx, "15376259491", "wrong"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expected invalid credential error, got %v", err)
	}
}

func TestResetPassword(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "jnmproxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := db.Migrate(ctx, store); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	service := NewService(repository.NewCredentialRepository(store))
	credential, err := service.CreateCredential(ctx, CreateCredentialInput{
		Username: "user",
		Password: "old-pass",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	if err := service.ResetPassword(ctx, credential.ID, "new-pass"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if _, err := service.Authenticate(ctx, "user", "old-pass"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("old password should fail, got %v", err)
	}
	if _, err := service.Authenticate(ctx, "user", "new-pass"); err != nil {
		t.Fatalf("new password should pass: %v", err)
	}
}
