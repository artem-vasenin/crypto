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
	"strings"
	"sync"
	"time"

	"universal-bybit-screener/models"
)

type Engine struct {
	cfg              models.BotConfig
	client           *http.Client
	baseURL          string
	mu               sync.Mutex
	positions        map[string]*models.PositionState
	closedHistory    map[string]*models.PositionState
	cooldowns        map[string]time.Time
	disabledTokens   map[string]bool
	leverageSetCache map[string]int
	cachedBalance    float64
	lastBalanceCheck time.Time
	wsEngine         *WSEngine
	targetSide       string
	processedExecs   map[string]time.Time
}

func NewEngine(cfg models.BotConfig, strategy string) *Engine {
	baseURL := "https://api.bybit.com"
	if cfg.Testnet {
		baseURL = "https://api-testnet.bybit.com"
	}

	targetSide := "Sell"
	if strings.ToLower(strategy) == "long" {
		targetSide = "Buy"
	}

	e := &Engine{
		cfg:              cfg,
		client:           &http.Client{Timeout: 5 * time.Second},
		baseURL:          baseURL,
		positions:        make(map[string]*models.PositionState),
		closedHistory:    make(map[string]*models.PositionState),
		cooldowns:        make(map[string]time.Time),
		disabledTokens:   make(map[string]bool),
		leverageSetCache: make(map[string]int),
		targetSide:       targetSide,
		processedExecs:   make(map[string]time.Time),
	}

	e.wsEngine = NewWSEngine(
		cfg.ApiKey,
		cfg.ApiSecret,
		cfg.Testnet,
		e.handlePositionClosedWS,
		e.handleBalanceUpdateWS,
		e.handleExecutionWS,
	)

	return e
}

func (e *Engine) InitWebSocket(ctx context.Context) error {
	if err := e.wsEngine.StartPublicTickerStream(ctx); err != nil {
		log.Printf("[WARN] Public WS failed: %v", err)
	}
	if err := e.wsEngine.StartPrivateStream(ctx); err != nil {
		log.Printf("[WARN] Private WS failed: %v", err)
	}
	return nil
}

func (e *Engine) handleBalanceUpdateWS(balance float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cachedBalance = balance
	e.lastBalanceCheck = time.Now()
}

func (e *Engine) handleExecutionWS(exec ExecutionLog) {
	e.mu.Lock()
	defer e.mu.Unlock()

	pos, exists := e.positions[exec.Symbol]
	if !exists || exec.ClosedSize <= 0 {
		return
	}

	pos.Size -= exec.ClosedSize
	if pos.Size <= 0.000001 {
		e.closedHistory[exec.Symbol] = pos
		delete(e.positions, exec.Symbol)
		e.cooldowns[exec.Symbol] = time.Now().Add(15 * time.Minute)
	}
}

func (e *Engine) handlePositionClosedWS(symbol string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if pos, active := e.positions[symbol]; active {
		e.closedHistory[symbol] = pos
		delete(e.positions, symbol)
		e.cooldowns[symbol] = time.Now().Add(15 * time.Minute)
	}
}

func (e *Engine) RefreshBalance(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	path := "/v5/account/wallet-balance"
	queryString := "accountType=UNIFIED"

	body, err := e.doSignedGET(ctx, path, queryString)
	if err != nil {
		return err
	}

	var res struct {
		Result struct {
			List []struct {
				Coin []struct {
					Coin                string `json:"coin"`
					AvailableToWithdraw string `json:"availableToWithdraw"`
				} `json:"coin"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &res); err == nil && len(res.Result.List) > 0 {
		for _, coin := range res.Result.List[0].Coin {
			if coin.Coin == "USDT" {
				bal, _ := strconv.ParseFloat(coin.AvailableToWithdraw, 64)
				e.cachedBalance = bal
				e.lastBalanceCheck = time.Now()
				return nil
			}
		}
	}
	return fmt.Errorf("failed to parse wallet balance")
}

func (e *Engine) ProcessCandidate(ctx context.Context, c models.Candidate, targetStrategy string) error {
	_ = e.wsEngine.SubscribeTicker(c.Symbol)

	side := "Sell"
	if strings.ToLower(targetStrategy) == "long" {
		side = "Buy"
	}

	if side != e.targetSide {
		return nil
	}

	e.mu.Lock()
	if _, active := e.positions[c.Symbol]; active || e.disabledTokens[c.Symbol] {
		e.mu.Unlock()
		return nil
	}

	if until, inCooldown := e.cooldowns[c.Symbol]; inCooldown {
		if time.Now().Before(until) {
			e.mu.Unlock()
			return nil
		}
		delete(e.cooldowns, c.Symbol)
	}

	sidePositionsCount := 0
	for _, pos := range e.positions {
		if pos.Side == e.targetSide || pos.Side == "PENDING" {
			sidePositionsCount++
		}
	}

	if sidePositionsCount >= e.cfg.MaxActivePositions {
		e.mu.Unlock()
		return nil
	}

	if e.cachedBalance < e.cfg.MarginPerTradeUSD {
		e.mu.Unlock()
		return fmt.Errorf("insufficient balance: available %.2f USD", e.cachedBalance)
	}

	e.positions[c.Symbol] = &models.PositionState{
		Symbol:   c.Symbol,
		Side:     "PENDING",
		OpenedAt: time.Now().UTC(),
	}
	e.mu.Unlock()

	orderPlaced := false
	defer func() {
		if !orderPlaced {
			e.mu.Lock()
			delete(e.positions, c.Symbol)
			e.cooldowns[c.Symbol] = time.Now().Add(3 * time.Minute)
			e.mu.Unlock()
		}
	}()

	res, ok := c.Strategies[targetStrategy]
	if !ok || res.Score < e.cfg.MinScore || res.Status == "reject" {
		return nil
	}

	bidPrice, askPrice, _, err := e.getLiveTicker(ctx, c.Symbol)
	if err != nil {
		return fmt.Errorf("ticker error for %s: %w", c.Symbol, err)
	}

	qtyStep, minQty, tickSize, minNotional, err := e.getInstrumentLimits(ctx, c.Symbol)
	if err != nil {
		return fmt.Errorf("limits error for %s: %w", c.Symbol, err)
	}

	entryPrice := askPrice
	if side == "Sell" {
		entryPrice = bidPrice
	}

	pivotLevel := c.Levels.NearestSupport
	if side == "Sell" {
		pivotLevel = c.Levels.NearestResistance
	}

	slPrice := CalculateDynamicStopLoss(side, entryPrice, pivotLevel, c.Indicators.ATR1h, 1.5, tickSize)
	tpPrice := CalculateDynamicTakeProfit(side, entryPrice, slPrice, 2.0, tickSize)

	if !ValidateStopLoss(side, entryPrice, slPrice, 4.0, c.Indicators.ATR1hPct) || !ValidateTakeProfit(side, entryPrice, tpPrice, 1.5) {
		return fmt.Errorf("validation failed for %s SL: %.4f | TP: %.4f", c.Symbol, slPrice, tpPrice)
	}

	targetLeverage := CalculateDynamicLeverage(c, targetStrategy, e.cfg.MaxLeverage)
	qty := CalculatePositionQty(e.cfg.MarginPerTradeUSD, targetLeverage, entryPrice, qtyStep, minQty, minNotional)

	if qty <= 0 {
		return fmt.Errorf("invalid qty for %s", c.Symbol)
	}

	_ = e.setTradeModeIsolated(ctx, c.Symbol, targetLeverage)
	_ = e.setLeverage(ctx, c.Symbol, targetLeverage)

	// Размещение ИСКЛЮЧИТЕЛЬНО Post-Only (Maker) Ордера
	orderID, err := e.placePostOnlyOrder(ctx, c.Symbol, side, qty, qtyStep, entryPrice, slPrice, tpPrice, tickSize)
	if err != nil {
		return fmt.Errorf("post-only limit order failed for %s: %w", c.Symbol, err)
	}

	orderPlaced = true
	log.Printf("[MAKER ENTRY] Symbol: %s | Side: %s | Qty: %.4f | Entry: %.4f | SL: %.4f | TP: %.4f | ID: %s",
		c.Symbol, side, qty, entryPrice, slPrice, tpPrice, orderID)

	e.mu.Lock()
	e.positions[c.Symbol] = &models.PositionState{
		Symbol:     c.Symbol,
		Side:       side,
		EntryPrice: entryPrice,
		Size:       qty,
		StopLoss:   slPrice,
		TakeProfit: tpPrice,
		OpenedAt:   time.Now().UTC(),
	}
	e.mu.Unlock()

	return nil
}

func (e *Engine) placePostOnlyOrder(ctx context.Context, symbol, side string, qty, qtyStep, price, sl, tp, tickSize float64) (string, error) {
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        side,
		"orderType":   "Limit",
		"qty":         FormatStep(qty, qtyStep),
		"price":       FormatStep(price, tickSize),
		"timeInForce": "PostOnly", // Защита от Taker Fees
		"stopLoss":    FormatStep(sl, tickSize),
		"takeProfit":  FormatStep(tp, tickSize),
		"slTriggerBy": "LastPrice",
		"tpTriggerBy": "LastPrice",
	}

	resp, err := e.doSignedPOST(ctx, "/v5/order/create", params, symbol)
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

func (e *Engine) getLiveTicker(ctx context.Context, symbol string) (bid, ask, last float64, err error) {
	reqURL := fmt.Sprintf("%s/v5/market/tickers?category=linear&symbol=%s", e.baseURL, symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, 0, err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Result struct {
			List []struct {
				Bid1Price string `json:"bid1Price"`
				Ask1Price string `json:"ask1Price"`
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}

	if json.Unmarshal(body, &res) != nil || len(res.Result.List) == 0 {
		return 0, 0, 0, fmt.Errorf("ticker parse error")
	}

	item := res.Result.List[0]
	bid, _ = strconv.ParseFloat(item.Bid1Price, 64)
	ask, _ = strconv.ParseFloat(item.Ask1Price, 64)
	last, _ = strconv.ParseFloat(item.LastPrice, 64)
	return bid, ask, last, nil
}

func (e *Engine) getInstrumentLimits(ctx context.Context, symbol string) (qtyStep, minQty, tickSize, minNotional float64, err error) {
	reqURL := fmt.Sprintf("%s/v5/market/instruments-info?category=linear&symbol=%s", e.baseURL, symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Result struct {
			List []struct {
				PriceFilter struct {
					TickSize string `json:"tickSize"`
				} `json:"priceFilter"`
				LotSizeFilter struct {
					QtyStep          string `json:"qtyStep"`
					MinOrderQty      string `json:"minOrderQty"`
					MinNotionalValue string `json:"minNotionalValue"`
				} `json:"lotSizeFilter"`
			} `json:"list"`
		} `json:"result"`
	}

	if json.Unmarshal(body, &res) != nil || len(res.Result.List) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("specs parse error")
	}

	item := res.Result.List[0]
	qtyStep, _ = strconv.ParseFloat(item.LotSizeFilter.QtyStep, 64)
	minQty, _ = strconv.ParseFloat(item.LotSizeFilter.MinOrderQty, 64)
	tickSize, _ = strconv.ParseFloat(item.PriceFilter.TickSize, 64)
	minNotional, _ = strconv.ParseFloat(item.LotSizeFilter.MinNotionalValue, 64)

	return qtyStep, minQty, tickSize, minNotional, nil
}

func (e *Engine) setTradeModeIsolated(ctx context.Context, symbol string, leverage int) error {
	levStr := strconv.Itoa(leverage)
	params := map[string]interface{}{
		"category":     "linear",
		"symbol":       symbol,
		"tradeMode":    1,
		"buyLeverage":  levStr,
		"sellLeverage": levStr,
	}
	_, err := e.doSignedPOST(ctx, "/v5/position/switch-isolated", params, symbol)
	return err
}

func (e *Engine) setLeverage(ctx context.Context, symbol string, leverage int) error {
	levStr := strconv.Itoa(leverage)
	params := map[string]interface{}{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  levStr,
		"sellLeverage": levStr,
	}
	_, err := e.doSignedPOST(ctx, "/v5/position/set-leverage", params, symbol)
	return err
}

func (e *Engine) doSignedGET(ctx context.Context, path, queryString string) ([]byte, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"
	rawSignature := timestamp + e.cfg.ApiKey + recvWindow + queryString

	h := hmac.New(sha256.New, []byte(e.cfg.ApiSecret))
	h.Write([]byte(rawSignature))
	signature := hex.EncodeToString(h.Sum(nil))

	fullURL := fmt.Sprintf("%s%s?%s", e.baseURL, path, queryString)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-BAPI-API-KEY", e.cfg.ApiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (e *Engine) doSignedPOST(ctx context.Context, path string, payload map[string]interface{}, symbol string) ([]byte, error) {
	jsonBody, _ := json.Marshal(payload)
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"
	rawSignature := timestamp + e.cfg.ApiKey + recvWindow + string(jsonBody)

	h := hmac.New(sha256.New, []byte(e.cfg.ApiSecret))
	h.Write([]byte(rawSignature))
	signature := hex.EncodeToString(h.Sum(nil))

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
	body, _ := io.ReadAll(resp.Body)

	var apiRes struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	_ = json.Unmarshal(body, &apiRes)

	if apiRes.RetCode != 0 && apiRes.RetCode != 110043 && apiRes.RetCode != 110026 {
		return nil, fmt.Errorf("bybit api code=%d: %s", apiRes.RetCode, apiRes.RetMsg)
	}

	return body, nil
}

func (e *Engine) LogActivePositions(ctx context.Context)                                {}
func (e *Engine) UpdateTrailingStops(ctx context.Context, symbol string, price float64) {}
