//go:build !with_utls

package singbox

func UTLSEnabled() bool {
	return false
}
