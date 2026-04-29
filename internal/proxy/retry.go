package proxy

import (
	"errors"

	"github.com/jnmproxy/jnmproxy/internal/cache"
)

const defaultMaxAttemptsPerRequest = 3
const directNodeName = "DIRECT"

func normalizeMaxAttempts(maxAttempts int) int {
	if maxAttempts <= 0 {
		return defaultMaxAttemptsPerRequest
	}
	return maxAttempts
}

func shouldFallbackDirect(err error) bool {
	return errors.Is(err, cache.ErrNoCandidateNodes)
}

func directSelection(store *cache.Store, username string) (cache.Selection, error) {
	credential, ok := store.Credential(username)
	if !ok || !credential.Enabled {
		return cache.Selection{}, cache.ErrCredentialNotFound
	}
	return cache.Selection{
		Credential: credential,
		Node: cache.NodeSnapshot{
			Name:     directNodeName,
			Protocol: "direct",
		},
		Direct: true,
	}, nil
}

func directAttempt(success bool, err error) RequestAttempt {
	attempt := RequestAttempt{NodeName: directNodeName, Success: success}
	if err != nil {
		attempt.Error = err.Error()
	}
	return attempt
}

func reportSelectionFailure(store *cache.Store, selection cache.Selection, reason string) {
	if selection.Direct || selection.Node.ID == 0 {
		return
	}
	store.ReportNodeFailure(selection.Node.ID, reason)
}

func reportSelectionSuccess(store *cache.Store, selection cache.Selection) {
	if selection.Direct || selection.Node.ID == 0 {
		return
	}
	store.ReportNodeSuccess(selection.Node.ID)
}
