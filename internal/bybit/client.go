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

const linear = "linear"

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
	return &Client{baseURL: c.BaseURL, httpClient: &http.Client{Timeout: c.HTTPTimeout}, maxRetries: c.MaxRetries, retryDelay: c.RetryDelay}
}

// doGET централизует HTTP, timeout, context, retry и проверку конверта Bybit.
func (c *Client) doGET(ctx context.Context, path string, q url.Values, dst any) error {
	requestURL := c.baseURL + path + "?" + q.Encode()
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var env struct {
					RetCode int             `json:"retCode"`
					RetMsg  string          `json:"retMsg"`
					Result  json.RawMessage `json:"result"`
				}
				if err = json.Unmarshal(body, &env); err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				if env.RetCode != 0 {
					return fmt.Errorf("bybit retCode=%d: %s", env.RetCode, env.RetMsg)
				}
				if err = json.Unmarshal(env.Result, dst); err != nil {
					return fmt.Errorf("decode result: %w", err)
				}
				return nil
			} else {
				lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			}
		} else {
			lastErr = err
		}
		if attempt < c.maxRetries {
			t := time.NewTimer(c.retryDelay * time.Duration(attempt+1))
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	}
	return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}
