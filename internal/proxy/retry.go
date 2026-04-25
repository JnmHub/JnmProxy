package proxy

const defaultMaxAttemptsPerRequest = 3

func normalizeMaxAttempts(maxAttempts int) int {
	if maxAttempts <= 0 {
		return defaultMaxAttemptsPerRequest
	}
	return maxAttempts
}
