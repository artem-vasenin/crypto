// internal/execution/engine.go
package execution

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"universal-bybit-screener/models"
)

type Engine struct {
	cfg       models.BotConfig
	client    *http.Client
	baseURL   string
	mu        sync.Mutex
	positions map[string]*models.PositionState
}

func NewEngine(cfg models.BotConfig) *Engine {
	baseURL := "https://api.bybit.com"
	if cfg.Testnet {
		baseURL = "https://api-testnet.bybit.com"
	}
	return &Engine{
		cfg:       cfg,
		client:    &http.Client{Timeout: 10 * time.Second},
		baseURL:   baseURL,
		positions: make(map[string]*models.PositionState),
	}
}

// ProcessCandidate анализирует кандидатов и исполняет вход при превышении MinScore
func (e *Engine) ProcessCandidate(ctx context.Context, c models.Candidate, targetStrategy string) error {
	e.mu.Lock()
	if _, active := e.positions[c.Symbol]; active {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	res, ok := c.Strategies[targetStrategy]
	if !ok || res.Score < e.cfg.MinScore || res.Status == "reject" {
		return nil
	}

	side := ""
	if targetStrategy == "long" {
		side = "Buy"
	} else if targetStrategy == "short" {
		side = "Sell"
	} else {
		return nil
	}

	log.Printf("[EXECUTION] Signal accepted for %s | Strategy: %s | Score: %.2f | Side: %s",
		c.Symbol, targetStrategy, res.Score, side)

	// 1. Установка изолированного плеча
	if err := e.setLeverage(ctx, c.Symbol, e.cfg.MaxLeverage); err != nil {
		log.Printf("[WARN] Failed to set leverage for %s: %v", c.Symbol, err)
	}

	// 2. Расчет точности объема ордера
	qty := CalculatePositionQty(e.cfg.MarginPerTradeUSD, e.cfg.MaxLeverage, c.Market.Price, 0.001)
	if qty <= 0 {
		return fmt.Errorf("calculated qty is too low for symbol %s", c.Symbol)
	}

	// 3. Вычисление Stop Loss
	slPrice := 0.0
	if side == "Buy" {
		if c.Levels.NearestSupport > 0 && c.Levels.NearestSupport < c.Market.Price {
			slPrice = c.Levels.NearestSupport
		} else {
			slPrice = c.Market.Price * (1 - (e.cfg.TrailingPct / 100))
		}
	} else {
		if c.Levels.NearestResistance > 0 && c.Levels.NearestResistance > c.Market.Price {
			slPrice = c.Levels.NearestResistance
		} else {
			slPrice = c.Market.Price * (1 + (e.cfg.TrailingPct / 100))
		}
	}

	if !ValidateStopLoss(side, c.Market.Price, slPrice, 0.2) {
		return fmt.Errorf("stop loss validation failed for %s (entry: %.4f, sl: %.4f)", c.Symbol, c.Market.Price, slPrice)
	}

	// 4. Отправка рыночного ордера с привязанным Stop-Loss
	orderID, err := e.placeMarketOrder(ctx, c.Symbol, side, qty, slPrice)
	if err != nil {
		return fmt.Errorf("order execution failed for %s: %w", c.Symbol, err)
	}

	log.Printf("[SUCCESS] Position opened: %s %s | Qty: %.4f | SL: %.4f | OrderID: %s",
		c.Symbol, side, qty, slPrice, orderID)

	e.mu.Lock()
	e.positions[c.Symbol] = &models.PositionState{
		Symbol:       c.Symbol,
		Side:         side,
		EntryPrice:   c.Market.Price,
		Size:         qty,
		StopLoss:     slPrice,
		HighestPrice: c.Market.Price,
		LowestPrice:  c.Market.Price,
		OpenedAt:     time.Now().UTC(),
	}
	e.mu.Unlock()

	return nil
}

// UpdateTrailingStops выполняет динамическое подтягивание Stop-Loss
func (e *Engine) UpdateTrailingStops(ctx context.Context, symbol string, currentPrice float64) {
	e.mu.Lock()
	pos, active := e.positions[symbol]
	if !active {
		e.mu.Unlock()
		return
	}

	updatedSL := 0.0
	shouldUpdate := false

	if pos.Side == "Buy" {
		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
			newSL := currentPrice * (1 - (e.cfg.TrailingPct / 100))
			if newSL > pos.StopLoss {
				pos.StopLoss = newSL
				updatedSL = newSL
				shouldUpdate = true
			}
		}
	} else if pos.Side == "Sell" {
		if currentPrice < pos.LowestPrice || pos.LowestPrice == 0 {
			pos.LowestPrice = currentPrice
			newSL := currentPrice * (1 + (e.cfg.TrailingPct / 100))
			if pos.StopLoss == 0 || newSL < pos.StopLoss {
				pos.StopLoss = newSL
				updatedSL = newSL
				shouldUpdate = true
			}
		}
	}
	e.mu.Unlock()

	if shouldUpdate {
		log.Printf("[TRAILING] Updating Trailing Stop for %s -> New SL: %.4f", symbol, updatedSL)
		if err := e.setTradingStop(ctx, symbol, pos.Side, updatedSL); err != nil {
			log.Printf("[ERROR] Failed to update SL on exchange for %s: %v", symbol, err)
		}
	}
}

func (e *Engine) setLeverage(ctx context.Context, symbol string, leverage int) error {
	levStr := strconv.Itoa(leverage)
	params := map[string]interface{}{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  levStr,
		"sellLeverage": levStr,
	}
	_, err := e.doSignedPOST(ctx, "/v5/position/set-leverage", params)
	return err
}

func (e *Engine) placeMarketOrder(ctx context.Context, symbol, side string, qty, sl float64) (string, error) {
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        side,
		"orderType":   "Market",
		"qty":         strconv.FormatFloat(qty, 'f', 4, 64),
		"timeInForce": "GTC",
		"stopLoss":    strconv.FormatFloat(sl, 'f', 4, 64),
	}
	resp, err := e.doSignedPOST(ctx, "/v5/order/create", params)
	if err != nil {
		return "", err
	}

	var res struct {
		Result struct {
			OrderId string `json:"orderId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &res); err != nil {
		return "", err
	}
	return res.Result.OrderId, nil
}

func (e *Engine) setTradingStop(ctx context.Context, symbol, side string, sl float64) error {
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"stopLoss":    strconv.FormatFloat(sl, 'f', 4, 64),
		"positionIdx": 0,
	}
	_, err := e.doSignedPOST(ctx, "/v5/position/trading-stop", params)
	return err
}

func (e *Engine) doSignedPOST(ctx context.Context, path string, payload map[string]interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"
	rawSignature := timestamp + e.cfg.ApiKey + recvWindow + string(jsonBody)

	h := hmac.New(sha256.New, []byte(e.cfg.ApiSecret))
	h.Write([]byte(rawSignature))
	signature := hex.EncodeToString(h.Sum(nil))

	// ИСПРАВЛЕНИЕ: Передача массива байт через bytes.NewReader для безопасного формирования HTTP Body
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BAPI-API-KEY", e.cfg.ApiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiRes struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if err := json.Unmarshal(body, &apiRes); err != nil {
		return nil, err
	}

	if apiRes.RetCode != 0 && apiRes.RetCode != 110043 {
		return nil, fmt.Errorf("bybit api error code=%d: %s", apiRes.RetCode, apiRes.RetMsg)
	}

	return body, nil
}
