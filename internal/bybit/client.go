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

// recvWindow — сколько миллисекунд Bybit допускает
// между timestamp запроса и временем сервера.
//
// Bybit использует это значение при проверке подписи.
const recvWindow = "5000"

// Client — REST-клиент Bybit.
//
// Он хранит:
//   - адрес API
//   - API key
//   - secret
//   - HTTP client
type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
}

// NewClient создаёт REST-клиент.
//
// Мы передаём сюда готовую конфигурацию.
// Поэтому client.go не знает, Testnet это или Mainnet.
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

// buildSignaturePayload формирует строку,
// которую необходимо подписать HMAC.
//
// Для GET-запроса Bybit использует:
//
// timestamp + apiKey + recvWindow + queryString
func buildSignaturePayload(
	timestamp string,
	apiKey string,
	recvWindow string,
	queryString string,
) string {
	return timestamp +
		apiKey +
		recvWindow +
		queryString
}

// generateSignature создаёт HMAC-SHA256 подпись.
//
// Secret никогда не должен попадать в лог.
func generateSignature(
	secret string,
	payload string,
) string {
	mac := hmac.New(
		sha256.New,
		[]byte(secret),
	)

	// hash.Hash.Write всегда возвращает ошибку,
	// но для HMAC она фактически невозможна.
	//
	// Поэтому здесь сознательно игнорируем её.
	_, _ = mac.Write([]byte(payload))

	return hex.EncodeToString(mac.Sum(nil))
}

// GetPositions получает открытые linear USDT позиции.
//
// Используется endpoint:
//
//	GET /v5/position/list
//
// Для нашего monitor это первоначальная синхронизация.
//
// После неё актуальные изменения приходят через WebSocket.
func (c *Client) GetPositions() ([]Position, error) {
	queryString := "category=linear&settleCoin=USDT"

	// Bybit ожидает timestamp в миллисекундах.
	timestamp := time.Now().UnixMilli()

	timestampStr := strconv.FormatInt(
		timestamp,
		10,
	)

	// Формируем строку для подписи.
	payload := buildSignaturePayload(
		timestampStr,
		c.apiKey,
		recvWindow,
		queryString,
	)

	signature := generateSignature(
		c.apiSecret,
		payload,
	)

	url := c.baseURL +
		"/v5/position/list?" +
		queryString

	req, err := http.NewRequest(
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Передаём параметры авторизации в HTTP headers.
	req.Header.Set(
		"X-BAPI-API-KEY",
		c.apiKey,
	)

	req.Header.Set(
		"X-BAPI-TIMESTAMP",
		timestampStr,
	)

	req.Header.Set(
		"X-BAPI-SIGN",
		signature,
	)

	req.Header.Set(
		"X-BAPI-RECV-WINDOW",
		recvWindow,
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// HTTP 200 ещё не означает успешный Bybit API-запрос.
	//
	// Bybit возвращает собственный retCode.
	var response PositionResponse

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf(
			"decode Bybit response: %w",
			err,
		)
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
