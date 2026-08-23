package bybit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"bybit_monitor/internal/config"
)

type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
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

func buildSignaturePayload(
	timestamp string,
	apiKey string,
	recvWindow string,
	queryString string,
) string {
	return timestamp + apiKey + recvWindow + queryString
}

func generateSignature(secret string, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))

	_, _ = mac.Write([]byte(payload))

	signature := mac.Sum(nil)

	return hex.EncodeToString(signature)
}

func (c *Client) GetPositions() ([]Position, error) {
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

	var response PositionResponse

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("decode Bybit response: %w", err)
	}

	if response.RetCode != 0 {
		return nil, fmt.Errorf(
			"Bybit API error %d: %s",
			response.RetCode,
			response.RetMsg,
		)
	}

	return response.Result.List, nil
}
