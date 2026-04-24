package proxy

import (
	"github.com/jnmproxy/jnmproxy/internal/auth"
	"github.com/jnmproxy/jnmproxy/internal/cache"
)

func verifyCachedCredential(store *cache.Store, username string, password string) bool {
	credential, ok := store.Credential(username)
	if !ok || !credential.Enabled {
		return false
	}
	return auth.VerifyPassword(credential.PasswordHash, password)
}
