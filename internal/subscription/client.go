package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type FetchResult struct {
	Body       []byte
	StatusCode int
	Usage      UsageInfo
}

type Client struct {
	httpClient *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient}
}

func (client *Client) Fetch(ctx context.Context, rawURL string, userAgent string, timeout time.Duration) (*FetchResult, error) {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	if userAgent == "" {
		userAgent = "clash/1.18.0"
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create subscription request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read subscription body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("subscription returned http %d", resp.StatusCode)
	}

	return &FetchResult{
		Body:       body,
		StatusCode: resp.StatusCode,
		Usage:      ParseUserInfoHeader(resp.Header.Get("subscription-userinfo")),
	}, nil
}
