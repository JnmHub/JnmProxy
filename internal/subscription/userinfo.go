package subscription

import (
	"strconv"
	"strings"
	"time"
)

type UsageInfo struct {
	UploadBytes   *int64
	DownloadBytes *int64
	TotalBytes    *int64
	ExpireAt      string
}

func ParseUserInfoHeader(header string) UsageInfo {
	var usage UsageInfo
	parts := strings.Split(header, ";")
	for _, part := range parts {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "upload":
			usage.UploadBytes = int64Ptr(parsed)
		case "download":
			usage.DownloadBytes = int64Ptr(parsed)
		case "total":
			usage.TotalBytes = int64Ptr(parsed)
		case "expire":
			if parsed > 0 {
				usage.ExpireAt = time.Unix(parsed, 0).UTC().Format(time.RFC3339)
			}
		}
	}
	return usage
}

func int64Ptr(value int64) *int64 {
	return &value
}
