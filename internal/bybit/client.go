package bybit

import (
	"bybit_monitor/internal/config"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
}

func generateSignature(secret string, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := mac.Sum(nil)
	return hex.EncodeToString(signature)
}

func buildSignaturePayload(timestamp, apiKey, recvWindow, queryString string) string {
	return timestamp + apiKey + recvWindow + queryString
}

func NewClient(cfg config.ByBit) *Client {
	return &Client{
		baseURL:   cfg.BaseURL,
		apiKey:    cfg.APIKey,
		apiSecret: cfg.Secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetPositions() ([]byte, error) {
	const recvWindow = "5000"

	queryString := "category=linear&settleCoin=USDT"

	timestamp := time.Now().UnixMilli()
	timestampStr := strconv.FormatInt(timestamp, 10)

	payload := buildSignaturePayload(
		timestampStr,
		c.apiKey,
		recvWindow,
		queryString,
	)

	signature := generateSignature(c.apiSecret, payload)

	url := c.baseURL + "/v5/position/list?" + queryString

	req, err := http.NewRequest(
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-BAPI-API-KEY", c.apiKey)
	req.Header.Set("X-BAPI-TIMESTAMP", timestampStr)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
