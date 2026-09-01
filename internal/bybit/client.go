// internal/bybit/client.go
package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

type ClientConfig struct {
	BaseURL     string
	HTTPTimeout time.Duration
	MaxRetries  int
	RetryDelay  time.Duration
}

func NewClient(c ClientConfig) *Client {
	return &Client{
		baseURL: c.BaseURL,
		httpClient: &http.Client{
			Timeout: c.HTTPTimeout,
		},
		maxRetries: c.MaxRetries,
		retryDelay: c.RetryDelay,
	}
}

// doGET выполнят HTTP GET с логикой повторных попыток (retries) и валидацией Bybit Envelope
func (c *Client) doGET(ctx context.Context, path string, q url.Values, dst any) error {
	requestURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, q.Encode())
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return fmt.Errorf("http request creation failed: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()

			if readErr != nil {
				lastErr = fmt.Errorf("failed to read response body: %w", readErr)
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var env struct {
					RetCode int             `json:"retCode"`
					RetMsg  string          `json:"retMsg"`
					Result  json.RawMessage `json:"result"`
				}

				if err = json.Unmarshal(body, &env); err != nil {
					return fmt.Errorf("json response decode failed: %w", err)
				}

				if env.RetCode != 0 {
					return fmt.Errorf("bybit api error retCode=%d: %s", env.RetCode, env.RetMsg)
				}

				if err = json.Unmarshal(env.Result, dst); err != nil {
					return fmt.Errorf("json result payload decode failed: %w", err)
				}

				return nil
			} else {
				lastErr = fmt.Errorf("HTTP status %d: %s", resp.StatusCode, string(body))
			}
		} else {
			lastErr = err
		}

		if attempt < c.maxRetries {
			backoff := c.retryDelay * time.Duration(attempt+1)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}
