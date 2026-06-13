package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPClient struct {
	BaseURL string
	Client  *http.Client
	Headers map[string]string
}

func NewHTTPClient(baseURL string, timeout time.Duration) HTTPClient {
	return HTTPClient{
		BaseURL: trimTrailingSlash(baseURL),
		Client:  &http.Client{Timeout: timeout},
	}
}

func NewHTTPClientWithHeaders(baseURL string, timeout time.Duration, headers map[string]string) HTTPClient {
	client := NewHTTPClient(baseURL, timeout)
	client.Headers = headers
	return client
}

func (c HTTPClient) GetJSON(ctx context.Context, path string, out any) (time.Duration, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return 0, err
	}
	for name, value := range c.Headers {
		req.Header.Set(name, value)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, StatusError{Path: path, StatusCode: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

type StatusError struct {
	Path       string
	StatusCode int
}

func (e StatusError) Error() string {
	return fmt.Sprintf("GET %s returned HTTP %d", e.Path, e.StatusCode)
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
