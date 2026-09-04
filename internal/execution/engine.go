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
	"math"
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
		client:           &http.Client{Timeout: 10 * time.Second},
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

func (e *Engine) handleBalanceUpdateWS(balance float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cachedBalance = balance
	e.lastBalanceCheck = time.Now()
	log.Printf("[WS BALANCE] Push update: Available USDT: %.2f USD", balance)
}

func (e *Engine) handleExecutionWS(exec ExecutionLog) {
	e.mu.Lock()
	defer e.mu.Unlock()

	pos, exists := e.positions[exec.Symbol]
	if !exists {
		return
	}

	if exec.ClosedSize <= 0 {
		return
	}

	entryPrice := pos.EntryPrice
	var grossPnL float64
	if pos.Side == "Buy" {
		grossPnL = (exec.ExecPrice - entryPrice) * exec.ClosedSize
	} else if pos.Side == "Sell" {
		grossPnL = (entryPrice - exec.ExecPrice) * exec.ClosedSize
	}

	netPnL := grossPnL - exec.ExecFee

	log.Printf("[TRADE CLOSED WS] %s %s | Closed Qty: %.4f | Entry: %.4f | Exit: %.4f | Fee: %.4f USDT | Net PnL: %.4f USDT | ExecType: %s",
		exec.Symbol, pos.Side, exec.ClosedSize, entryPrice, exec.ExecPrice, exec.ExecFee, netPnL, exec.ExecType)

	pos.Size -= exec.ClosedSize

	if pos.Size <= 0.000001 {
		e.closedHistory[exec.Symbol] = pos
		delete(e.positions, exec.Symbol)
		e.cooldowns[exec.Symbol] = time.Now().Add(30 * time.Minute)
	}
}

func (e *Engine) InitWebSocket(ctx context.Context) error {
	if err := e.wsEngine.StartPublicTickerStream(ctx); err != nil {
		log.Printf("[WARN] Failed to start public WS stream: %v", err)
	}
	if err := e.wsEngine.StartPrivateStream(ctx); err != nil {
		log.Printf("[WARN] Failed to start private WS stream: %v", err)
	}
	return nil
}

func (e *Engine) handlePositionClosedWS(symbol string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if pos, active := e.positions[symbol]; active {
		log.Printf("[CLEANUP WS] Position %s closed on exchange. Activating 30m Post-Trade Cooldown.", symbol)
		e.closedHistory[symbol] = pos
		delete(e.positions, symbol)
		e.cooldowns[symbol] = time.Now().Add(30 * time.Minute)
	}
}

func (e *Engine) RefreshBalance(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	bal, err := e.fetchWalletBalance(ctx)
	if err != nil {
		return err
	}

	e.cachedBalance = bal
	e.lastBalanceCheck = time.Now()
	log.Printf("[BALANCE REST] Wallet USDT Available: %.2f USD", bal)
	return nil
}

func (e *Engine) syncClosedPositionsREST(ctx context.Context) {
	startTime := time.Now().Add(-10 * time.Minute).UnixMilli()
	path := "/v5/position/closed-pnl"
	queryString := fmt.Sprintf("category=linear&limit=10&startTime=%d", startTime)

	body, err := e.doSignedGET(ctx, path, queryString)
	if err != nil {
		return
	}

	var res struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				OrderId       string `json:"orderId"`
				Symbol        string `json:"symbol"`
				OrderSide     string `json:"orderSide"`
				ClosedPnl     string `json:"closedPnl"`
				AvgEntryPrice string `json:"avgEntryPrice"`
				AvgExitPrice  string `json:"avgExitPrice"`
				ClosedSize    string `json:"closedSize"`
				ExecType      string `json:"execType"`
				UpdatedTime   string `json:"updatedTime"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &res); err != nil || res.RetCode != 0 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for id, t := range e.processedExecs {
		if now.Sub(t) > 15*time.Minute {
			delete(e.processedExecs, id)
		}
	}

	for _, item := range res.Result.List {
		dedupKey := item.Symbol + "_" + item.OrderId
		if _, processed := e.processedExecs[dedupKey]; processed {
			continue
		}

		pos, exists := e.positions[item.Symbol]
		if !exists {
			continue
		}

		pnl, _ := strconv.ParseFloat(item.ClosedPnl, 64)
		entry, _ := strconv.ParseFloat(item.AvgEntryPrice, 64)
		exit, _ := strconv.ParseFloat(item.AvgExitPrice, 64)
		size, _ := strconv.ParseFloat(item.ClosedSize, 64)

		log.Printf("[TRADE CLOSED REST RESTORE] %s | Qty: %.4f | Entry: %.4f | Exit: %.4f | Net PnL: %.4f USDT | ExecType: %s",
			item.Symbol, size, entry, exit, pnl, item.ExecType)

		e.processedExecs[dedupKey] = now
		e.closedHistory[item.Symbol] = pos
		delete(e.positions, item.Symbol)
		e.cooldowns[item.Symbol] = time.Now().Add(30 * time.Minute)
	}
}

func (e *Engine) CheckStalePositions(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	staleThreshold := 60 * time.Minute

	for symbol, pos := range e.positions {
		if pos.Side == "PENDING" || pos.Side != e.targetSide {
			continue
		}

		if pos.OpenedAt.IsZero() {
			pos.OpenedAt = now.Add(-30 * time.Minute)
		}

		if now.Sub(pos.OpenedAt) > staleThreshold {
			currentPrice := pos.EntryPrice
			if wsPrice, ok := e.wsEngine.GetLatestPrice(symbol); ok && wsPrice > 0 {
				currentPrice = wsPrice
			}

			pnlPct := 0.0
			if pos.Side == "Buy" {
				pnlPct = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100.0
			} else if pos.Side == "Sell" {
				pnlPct = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100.0
			}

			if (pnlPct >= 0.15 && pnlPct < 0.8) || (pnlPct <= -0.6 && pnlPct > -1.1) {
				log.Printf("[TIME-STOP] Liquidating stale position %s %s (Hold Time: %s, PnL: %.2f%%)",
					symbol, pos.Side, now.Sub(pos.OpenedAt).Round(time.Minute), pnlPct)

				go func(sym, side string, qty float64) {
					closeSide := "Sell"
					if side == "Sell" {
						closeSide = "Buy"
					}

					params := map[string]interface{}{
						"category":    "linear",
						"symbol":      sym,
						"side":        closeSide,
						"orderType":   "Market",
						"qty":         strconv.FormatFloat(qty, 'f', -1, 64),
						"reduceOnly":  true,
						"timeInForce": "GTC",
					}
					_, err := e.doSignedPOST(ctx, "/v5/order/create", params, sym)
					if err != nil {
						log.Printf("[ERROR] Time-Stop execution failed for %s: %v", sym, err)
					}
				}(symbol, pos.Side, pos.Size)
			}
		}
	}
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
	if _, active := e.positions[c.Symbol]; active {
		e.mu.Unlock()
		return nil
	}

	if e.disabledTokens[c.Symbol] {
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

	requiredMarginWithBuffer := e.cfg.MarginPerTradeUSD * 1.02
	if e.cachedBalance < requiredMarginWithBuffer {
		e.mu.Unlock()
		return fmt.Errorf("insufficient balance with fee buffer: required %.2f USDT, available %.2f USDT", requiredMarginWithBuffer, e.cachedBalance)
	}

	e.cachedBalance -= e.cfg.MarginPerTradeUSD
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
			e.cachedBalance += e.cfg.MarginPerTradeUSD
			e.cooldowns[c.Symbol] = time.Now().Add(5 * time.Minute)
			e.mu.Unlock()
		}
	}()

	res, ok := c.Strategies[targetStrategy]
	if !ok || res.Score < e.cfg.MinScore || res.Status == "reject" {
		return nil
	}

	currentPrice := 0.0
	bidPrice, askPrice, _, err := e.getLiveTicker(ctx, c.Symbol)
	if err != nil {
		if wsPrice, ok := e.wsEngine.GetLatestPrice(c.Symbol); ok {
			currentPrice = wsPrice
			bidPrice = wsPrice
			askPrice = wsPrice
		} else {
			return fmt.Errorf("failed to fetch live price for %s: %w", c.Symbol, err)
		}
	} else {
		spreadPct := (askPrice - bidPrice) / askPrice * 100.0
		if spreadPct > 0.10 {
			return fmt.Errorf("rejected %s: spread %.3f%% exceeds max 0.10%% limit", c.Symbol, spreadPct)
		}
		if side == "Buy" {
			currentPrice = askPrice
		} else {
			currentPrice = bidPrice
		}
	}

	qtyStep, minQty, tickSize, minNotional, err := e.getInstrumentLimits(ctx, c.Symbol)
	if err != nil {
		return fmt.Errorf("failed to fetch instrument specs for %s: %w", c.Symbol, err)
	}

	slPct := 0.018
	slDist := currentPrice * slPct
	slPrice := 0.0

	if side == "Buy" {
		slPrice = currentPrice - slDist
		if c.Levels.NearestSupport > 0 && c.Levels.NearestSupport < currentPrice {
			supDistPct := (currentPrice - c.Levels.NearestSupport) / currentPrice
			if supDistPct >= 0.012 && supDistPct <= 0.023 {
				slPrice = c.Levels.NearestSupport
			}
		}
	} else {
		slPrice = currentPrice + slDist
		if c.Levels.NearestResistance > currentPrice {
			resDistPct := (c.Levels.NearestResistance - currentPrice) / currentPrice
			if resDistPct >= 0.012 && resDistPct <= 0.023 {
				slPrice = c.Levels.NearestResistance
			}
		}
	}
	slPrice = RoundToStep(slPrice, tickSize)

	if !ValidateStopLoss(side, currentPrice, slPrice, 2.5) {
		return fmt.Errorf("stop loss validation failed for %s (entry: %.4f, sl: %.4f)", c.Symbol, currentPrice, slPrice)
	}

	targetLeverage := CalculateDynamicLeverage(c, targetStrategy, e.cfg.MaxLeverage)
	qty := CalculatePositionQty(e.cfg.MarginPerTradeUSD, targetLeverage, currentPrice, qtyStep, minQty, minNotional)

	if qty <= 0 {
		return fmt.Errorf("calculated qty (0) is invalid for %s (minQty: %f, minNotional: %f)", c.Symbol, minQty, minNotional)
	}

	e.mu.Lock()
	cachedLev := e.leverageSetCache[c.Symbol]
	e.mu.Unlock()

	if cachedLev != targetLeverage {
		_ = e.setTradeModeIsolated(ctx, c.Symbol, targetLeverage)
		if err := e.setLeverage(ctx, c.Symbol, targetLeverage); err != nil {
			log.Printf("[WARN] Set leverage x%d for %s: %v", targetLeverage, c.Symbol, err)
		} else {
			e.mu.Lock()
			e.leverageSetCache[c.Symbol] = targetLeverage
			e.mu.Unlock()
		}
	}

	actualRiskDist := math.Abs(currentPrice - slPrice)
	minTPDist := currentPrice * 0.025
	tpDist := math.Max(actualRiskDist*1.8, minTPDist)
	tpPrice := 0.0

	if side == "Buy" {
		tpPrice = currentPrice + tpDist
		if c.Levels.NearestResistance > (currentPrice+minTPDist) && c.Levels.NearestResistance < tpPrice {
			tpPrice = c.Levels.NearestResistance
		}
	} else {
		tpPrice = currentPrice - tpDist
		if c.Levels.NearestSupport > 0 && c.Levels.NearestSupport < (currentPrice-minTPDist) && c.Levels.NearestSupport > tpPrice {
			tpPrice = c.Levels.NearestSupport
		}
	}
	tpPrice = RoundToStep(tpPrice, tickSize)

	if !ValidateTakeProfit(side, currentPrice, tpPrice, 1.2) {
		return fmt.Errorf("take profit validation failed for %s (entry: %.4f, tp: %.4f)", c.Symbol, currentPrice, tpPrice)
	}

	orderID, err := e.placeMarketOrder(ctx, c.Symbol, side, qty, qtyStep, slPrice, tpPrice, tickSize)
	if err != nil {
		return fmt.Errorf("order execution failed for %s: %w", c.Symbol, err)
	}

	orderPlaced = true

	log.Printf("[SUCCESS] Position opened: %s %s | Leverage: x%d | Price: %.4f | Qty: %s | SL: %s | TP: %s | OrderID: %s",
		c.Symbol, side, targetLeverage, currentPrice, FormatStep(qty, qtyStep), FormatStep(slPrice, tickSize), FormatStep(tpPrice, tickSize), orderID)

	e.mu.Lock()
	e.positions[c.Symbol] = &models.PositionState{
		Symbol:       c.Symbol,
		Side:         side,
		EntryPrice:   currentPrice,
		Size:         qty,
		StopLoss:     slPrice,
		HighestPrice: currentPrice,
		LowestPrice:  currentPrice,
		OpenedAt:     time.Now().UTC(),
	}
	e.mu.Unlock()

	return nil
}

func (e *Engine) UpdateTrailingStops(ctx context.Context, symbol string, currentPrice float64) {
	e.mu.Lock()
	pos, active := e.positions[symbol]
	if !active || pos.Side == "PENDING" || pos.Side != e.targetSide {
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()

	if wsPrice, ok := e.wsEngine.GetLatestPrice(symbol); ok && wsPrice > 0 {
		currentPrice = wsPrice
	}

	if !e.hasActivePosition(ctx, symbol) {
		log.Printf("[CLEANUP] Position %s closed on exchange. Activating 30m Post-Trade Cooldown.", symbol)
		e.mu.Lock()
		e.closedHistory[symbol] = pos
		delete(e.positions, symbol)
		e.cooldowns[symbol] = time.Now().Add(30 * time.Minute)
		e.mu.Unlock()
		return
	}

	_, _, tickSize, _, err := e.getInstrumentLimits(ctx, symbol)
	if err != nil {
		tickSize = 0.0001
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	updatedSL := 0.0
	shouldUpdate := false

	if pos.Side == "Buy" {
		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
			newSL := RoundToStep(currentPrice*(1-(e.cfg.TrailingPct/100)), tickSize)
			if newSL > pos.StopLoss && math.Abs(newSL-pos.StopLoss) >= tickSize {
				pos.StopLoss = newSL
				updatedSL = newSL
				shouldUpdate = true
			}
		}
	} else if pos.Side == "Sell" {
		if currentPrice < pos.LowestPrice || pos.LowestPrice == 0 {
			pos.LowestPrice = currentPrice
			newSL := RoundToStep(currentPrice*(1+(e.cfg.TrailingPct/100)), tickSize)
			if (pos.StopLoss == 0 || newSL < pos.StopLoss) && math.Abs(newSL-pos.StopLoss) >= tickSize {
				pos.StopLoss = newSL
				updatedSL = newSL
				shouldUpdate = true
			}
		}
	}

	if shouldUpdate {
		log.Printf("[TRAILING] Updating Trailing Stop for %s -> New SL: %s", symbol, FormatStep(updatedSL, tickSize))
		if err := e.setTradingStop(ctx, symbol, pos.Side, updatedSL, tickSize); err != nil {
			log.Printf("[ERROR] Failed to update SL on exchange for %s: %v", symbol, err)
		}
	}
}

func (e *Engine) LogActivePositions(ctx context.Context) {
	e.syncClosedPositionsREST(ctx)
	e.CheckStalePositions(ctx)

	path := "/v5/position/list"
	queryString := "category=linear&settleCoin=USDT"

	body, err := e.doSignedGET(ctx, path, queryString)
	if err != nil {
		return
	}

	var res struct {
		Result struct {
			List []struct {
				Symbol        string `json:"symbol"`
				Side          string `json:"side"`
				Size          string `json:"size"`
				AvgPrice      string `json:"avgPrice"`
				UnrealisedPnl string `json:"unrealisedPnl"`
				StopLoss      string `json:"stopLoss"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return
	}

	exchangeActive := make(map[string]bool)
	targetSideCount := 0
	totalSideUnrealizedPnL := 0.0
	now := time.Now().UTC()

	e.mu.Lock()

	for sym, pos := range e.positions {
		if pos.Side == "PENDING" && now.Sub(pos.OpenedAt) > 30*time.Second {
			log.Printf("[CLEANUP] Expired PENDING state for %s. Refunding margin.", sym)
			delete(e.positions, sym)
			e.cachedBalance += e.cfg.MarginPerTradeUSD
		}
	}

	for _, pos := range res.Result.List {
		size, _ := strconv.ParseFloat(pos.Size, 64)
		if size > 0 {
			pnl, _ := strconv.ParseFloat(pos.UnrealisedPnl, 64)
			avgPrice, _ := strconv.ParseFloat(pos.AvgPrice, 64)
			currentSL, _ := strconv.ParseFloat(pos.StopLoss, 64)

			if pos.Side == e.targetSide {
				exchangeActive[pos.Symbol] = true
				targetSideCount++
				totalSideUnrealizedPnL += pnl

				if state, exists := e.positions[pos.Symbol]; !exists || state.Side == "PENDING" {
					e.positions[pos.Symbol] = &models.PositionState{
						Symbol:       pos.Symbol,
						Side:         pos.Side,
						EntryPrice:   avgPrice,
						Size:         size,
						StopLoss:     currentSL,
						HighestPrice: avgPrice,
						LowestPrice:  avgPrice,
						OpenedAt:     time.Now().UTC(),
					}
				} else {
					if currentSL > 0 {
						state.StopLoss = currentSL
					}
				}

				log.Printf("[POS MONITOR] %s %s | Size: %s | Entry: %s | uPnL: %.4f USDT",
					pos.Symbol, pos.Side, pos.Size, pos.AvgPrice, pnl)
			}
		}
	}

	for sym, pos := range e.positions {
		if pos.Side != "PENDING" && pos.Side == e.targetSide && !exchangeActive[sym] {
			log.Printf("[SYNC CLEANUP] Removing ghost position %s from state. Activating 30m Post-Trade Cooldown.", sym)
			e.closedHistory[sym] = pos
			delete(e.positions, sym)
			e.cooldowns[sym] = time.Now().Add(30 * time.Minute)
		}
	}

	if time.Since(e.lastBalanceCheck) > 3*time.Minute {
		e.mu.Unlock()
		_ = e.RefreshBalance(ctx)
		e.mu.Lock()
	}

	balance := e.cachedBalance
	e.mu.Unlock()

	log.Printf("[SUMMARY] Strategy Target: %s | Active Positions: %d/%d | Wallet Balance: %.2f USDT | Target uPnL: %.4f USDT",
		e.targetSide, targetSideCount, e.cfg.MaxActivePositions, balance, totalSideUnrealizedPnL)
}

func (e *Engine) fetchWalletBalance(ctx context.Context) (float64, error) {
	path := "/v5/account/wallet-balance"
	queryString := "accountType=UNIFIED"

	body, err := e.doSignedGET(ctx, path, queryString)
	if err != nil {
		return 0, fmt.Errorf("wallet balance fetch failed: %w", err)
	}

	var res struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				Coin []struct {
					Coin                string `json:"coin"`
					AvailableToWithdraw string `json:"availableToWithdraw"`
					WalletBalance       string `json:"walletBalance"`
				} `json:"coin"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return 0, fmt.Errorf("failed to unmarshal balance JSON: %w", err)
	}

	if res.RetCode != 0 {
		return 0, fmt.Errorf("bybit balance api error code=%d msg=%s", res.RetCode, res.RetMsg)
	}

	if len(res.Result.List) == 0 {
		return 0.0, nil
	}

	for _, coin := range res.Result.List[0].Coin {
		if coin.Coin == "USDT" {
			if coin.AvailableToWithdraw != "" {
				bal, err := strconv.ParseFloat(coin.AvailableToWithdraw, 64)
				if err == nil && bal >= 0 {
					return bal, nil
				}
			}
			if coin.WalletBalance != "" {
				bal, err := strconv.ParseFloat(coin.WalletBalance, 64)
				if err == nil && bal >= 0 {
					return bal, nil
				}
			}
		}
	}

	return 0.0, nil
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, err
	}

	var res struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				Bid1Price string `json:"bid1Price"`
				Ask1Price string `json:"ask1Price"`
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &res); err != nil || len(res.Result.List) == 0 {
		return 0, 0, 0, fmt.Errorf("failed to parse ticker response for %s", symbol)
	}

	item := res.Result.List[0]
	bid, _ = strconv.ParseFloat(item.Bid1Price, 64)
	ask, _ = strconv.ParseFloat(item.Ask1Price, 64)
	last, _ = strconv.ParseFloat(item.LastPrice, 64)

	return bid, ask, last, nil
}

func (e *Engine) hasActivePosition(ctx context.Context, symbol string) bool {
	path := "/v5/position/list"
	queryString := fmt.Sprintf("category=linear&symbol=%s", symbol)

	body, err := e.doSignedGET(ctx, path, queryString)
	if err != nil {
		return false
	}

	var res struct {
		Result struct {
			List []struct {
				Size string `json:"size"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &res); err != nil || len(res.Result.List) == 0 {
		return false
	}

	size, _ := strconv.ParseFloat(res.Result.List[0].Size, 64)
	return size > 0
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	var res struct {
		RetCode int `json:"retCode"`
		Result  struct {
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

	if err := json.Unmarshal(body, &res); err != nil || len(res.Result.List) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("symbol info not found or unmarshal error: %w", err)
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

func (e *Engine) placeMarketOrder(ctx context.Context, symbol, side string, qty, qtyStep, sl, tp, tickSize float64) (string, error) {
	bid, ask, last, err := e.getLiveTicker(ctx, symbol)
	if err == nil && last > 0 {
		execPrice := ask
		if side == "Sell" {
			execPrice = bid
		}
		priceDevPct := math.Abs(execPrice-last) / last * 100.0
		if priceDevPct > 0.10 {
			return "", fmt.Errorf("rejected market order for %s: execution price deviation %.3f%% exceeds max 0.10%% limit", symbol, priceDevPct)
		}
	}

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        side,
		"orderType":   "Market",
		"qty":         FormatStep(qty, qtyStep),
		"timeInForce": "GTC",
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

// ФИКСИРОВАННЫЙ МЕТОД: Добавлено обязательное поле "symbol"
func (e *Engine) setTradingStop(ctx context.Context, symbol, side string, sl, tickSize float64) error {
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"stopLoss":    FormatStep(sl, tickSize),
		"positionIdx": 0,
	}
	_, err := e.doSignedPOST(ctx, "/v5/position/trading-stop", params, symbol)
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

	if apiRes.RetCode == 110126 {
		e.mu.Lock()
		e.disabledTokens[symbol] = true
		e.mu.Unlock()
		log.Printf("[BLACKLIST] Token %s disabled due to missing user agreement (code 110126)", symbol)
	}

	if apiRes.RetCode != 0 && apiRes.RetCode != 110043 && apiRes.RetCode != 110026 {
		return nil, fmt.Errorf("bybit api error code=%d: %s", apiRes.RetCode, apiRes.RetMsg)
	}

	return body, nil
}
