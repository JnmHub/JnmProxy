package proxy

import (
	"net/http"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/stats"
)

func recordStats(collector *stats.Collector, selection cache.Selection, uploadBytes int64, downloadBytes int64, success bool) {
	if collector == nil {
		return
	}
	collector.Record(stats.Event{
		CredentialID:   selection.Credential.ID,
		SubscriptionID: selection.Node.SubscriptionID,
		NodeID:         selection.Node.ID,
		GroupID:        selection.GroupID,
		UploadBytes:    uploadBytes,
		DownloadBytes:  downloadBytes,
		Success:        success,
	})
}

func requestUploadBytes(r *http.Request) int64 {
	if r.ContentLength > 0 {
		return r.ContentLength
	}
	return 0
}
