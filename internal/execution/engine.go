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
	"net/url"
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
	candidatesCache  map[string]models.Candidate
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
	if strings.EqualFold(strategy, "long") {
		targetSide = "Buy"
	}

	engine := &Engine{
		cfg:             cfg,
		client:          &http.Client{Timeout: 5 * time.Second},
		baseURL:         baseURL,
		positions:       make(map[string]*models.PositionState),
		closedHistory:   make(map[string]*models.PositionState),
		cooldowns:       make(map[string]time.Time),
		disabledTokens:  make(map[string]bool),
		candidatesCache: make(map[string]models.Candidate),
		targetSide:      targetSide,
		processedExecs:  make(map[string]time.Time),
	}

	engine.wsEngine = NewWSEngine(
		cfg.ApiKey,
		cfg.ApiSecret,
		cfg.Testnet,
		engine.handlePositionUpdateWS,
		engine.handleOrderUpdateWS,
		engine.handleExecutionWS,
		engine.handleBalanceUpdateWS,
	)
	return engine
}

func (e *Engine) InitWebSocket(ctx context.Context) error {
	if err := e.wsEngine.StartPublicTickerStream(ctx); err != nil {
		return fmt.Errorf("public websocket: %w", err)
	}
	if err := e.wsEngine.StartPrivateStream(ctx); err != nil {
		return fmt.Errorf("private websocket: %w", err)
	}
	return nil
}

func (e *Engine) RefreshState(ctx context.Context) error {
	if err := e.RefreshBalance(ctx); err != nil {
		return err
	}
	if err := e.refreshPositions(ctx); err != nil {
		return err
	}
	return e.refreshOpenOrders(ctx)
}

func (e *Engine) handleBalanceUpdateWS(balance float64) {
	e.mu.Lock()
	e.cachedBalance = balance
	e.lastBalanceCheck = time.Now()
	e.mu.Unlock()
}

func (e *Engine) handlePositionUpdateWS(update PositionUpdate) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if update.Size <= 0 {
		if pos, ok := e.positions[update.Symbol]; ok && !pos.Pending {
			e.closedHistory[update.Symbol] = pos
			delete(e.positions, update.Symbol)
			delete(e.candidatesCache, update.Symbol)
			e.cooldowns[update.Symbol] = time.Now().Add(15 * time.Minute)
		}
		return
	}

	pos, exists := e.positions[update.Symbol]
	if !exists {
		pos = &models.PositionState{
			Symbol:   update.Symbol,
			OpenedAt: time.Now().UTC(),
		}
		e.positions[update.Symbol] = pos
	}

	pos.Side = update.Side
	pos.Size = update.Size
	pos.EntryPrice = update.EntryPrice
	pos.Pending = false
	if pos.OpenedAt.IsZero() {
		pos.OpenedAt = time.Now().UTC()
	}
}

func (e *Engine) handleOrderUpdateWS(update OrderUpdate) {
	e.mu.Lock()
	defer e.mu.Unlock()

	pos, exists := e.positions[update.Symbol]
	if !exists || pos.OrderID != update.OrderID {
		return
	}

	switch update.Status {
	case "Filled":
		pos.Pending = false
	case "Cancelled", "Rejected", "Deactivated":
		if pos.Pending {
			delete(e.positions, update.Symbol)
			delete(e.candidatesCache, update.Symbol)
			e.cooldowns[update.Symbol] = time.Now().Add(3 * time.Minute)
		}
	}
}

func (e *Engine) handleExecutionWS(exec ExecutionLog) {
	if exec.ExecID != "" {
		e.mu.Lock()
		if _, seen := e.processedExecs[exec.ExecID]; seen {
			e.mu.Unlock()
			return
		}
		e.processedExecs[exec.ExecID] = time.Now()
		e.mu.Unlock()
	}

	if exec.ExecQty <= 0 && exec.ClosedSize <= 0 {
		return
	}

	log.Printf("[EXECUTION] %s %s qty=%.8f price=%.8f fee=%.8f closed=%.8f",
		exec.Symbol, exec.Side, exec.ExecQty, exec.ExecPrice, exec.ExecFee, exec.ClosedSize)

	if exec.ClosedSize > 0 {
		return
	}

	e.mu.Lock()
	candidate, hasCandidate := e.candidatesCache[exec.Symbol]
	pos, exists := e.positions[exec.Symbol]
	if !exists || !pos.Managed {
		e.mu.Unlock()
		return
	}

	// Process only executions belonging to the order created by this engine.
	// This is important when long and short bots run against the same account:
	// each bot receives the same private execution stream.
	if pos.OrderID != "" && exec.OrderID != "" && pos.OrderID != exec.OrderID {
		e.mu.Unlock()
		return
	}

	// The execution side must match the side selected by this bot. A mismatch
	// means that local state and the exchange state disagree, so trading for
	// this symbol is disabled until the situation is investigated.
	if exec.Side != e.targetSide || exec.Side != pos.Side {
		e.disabledTokens[exec.Symbol] = true
		log.Printf("[CRITICAL] execution side mismatch for %s: bot expects %s, local side=%s, exchange execution=%s; symbol disabled",
			exec.Symbol, e.targetSide, pos.Side, exec.Side)
		e.mu.Unlock()
		return
	}

	if !hasCandidate {
		e.mu.Unlock()
		return
	}

	orderID := exec.OrderID
	if orderID == "" {
		orderID = pos.OrderID
	}
	leverage := pos.Leverage
	snapshotNeeded := !pos.SnapshotSaved
	if snapshotNeeded {
		pos.SnapshotSaved = true
	}
	e.mu.Unlock()

	if snapshotNeeded {
		btcTrend, _ := e.wsEngine.GetBTCTrend15m()
		if err := SaveTradeSnapshot(
			exec.Symbol,
			exec.Side,
			exec.ExecPrice,
			exec.ExecQty,
			leverage,
			orderID,
			candidate,
			btcTrend,
			exec.ExecFee,
			exec.ExecID,
			exec.ExecTime,
		); err != nil {
			e.mu.Lock()
			if current, ok := e.positions[exec.Symbol]; ok {
				current.SnapshotSaved = false
			}
			e.mu.Unlock()
			log.Printf("[ERROR] failed to save trade snapshot for %s: %v", exec.Symbol, err)
		} else {
			log.Printf("[SNAPSHOT] saved entry snapshot for %s %s order=%s exec_price=%.8f exec_qty=%.8f",
				exec.Symbol, exec.Side, orderID, exec.ExecPrice, exec.ExecQty)
		}
	}

	if !posRiskAttached(e, exec.Symbol) {
		log.Printf("[CRITICAL] %s execution received but local risk state is not attached", exec.Symbol)
	}
}

func posRiskAttached(e *Engine, symbol string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	pos, ok := e.positions[symbol]
	return ok && pos.RiskAttached
}

func (e *Engine) calculateRiskLevels(side string, entryPrice float64, candidate models.Candidate, tickSize float64) (float64, float64, float64, error) {
	if entryPrice <= 0 || candidate.Indicators.ATR1h <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid entry price or ATR")
	}

	pivot := candidate.Levels.NearestSupport
	if side == "Sell" {
		pivot = candidate.Levels.NearestResistance
	}

	sl := CalculateDynamicStopLoss(side, entryPrice, pivot, candidate.Indicators.ATR1h, 1.5, tickSize)
	tp := CalculateDynamicTakeProfit(side, entryPrice, sl, 2, tickSize)
	if !ValidateStopLoss(side, entryPrice, sl, e.cfg.MaxStopLossPct, candidate.Indicators.ATR1hPct) {
		return 0, 0, 0, fmt.Errorf("stop loss %.4f is outside configured risk limit %.4f%%", sl, e.cfg.MaxStopLossPct)
	}

	costPct := EstimatedRoundTripCostPct(e.cfg.MakerFeeRate, e.cfg.TakerFeeRate, e.cfg.ExtraCostPct)
	netProfitPct := mathAbsPct(math.Abs(tp-entryPrice), entryPrice) - costPct
	if !ValidateTakeProfit(side, entryPrice, tp, e.cfg.MinNetProfitPct+costPct) || netProfitPct < e.cfg.MinNetProfitPct {
		return 0, 0, 0, fmt.Errorf("take profit net distance %.4f%% is below minimum %.4f%% after estimated costs", netProfitPct, e.cfg.MinNetProfitPct)
	}

	return sl, tp, costPct, nil
}

func (e *Engine) RefreshBalance(ctx context.Context) error {
	body, err := e.doSignedGET(ctx, "/v5/account/wallet-balance", url.Values{
		"accountType": {"UNIFIED"},
	}.Encode())
	if err != nil {
		return err
	}

	var res struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				TotalAvailableBalance string `json:"totalAvailableBalance"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("wallet response parse error: %w", err)
	}
	if res.RetCode != 0 {
		return fmt.Errorf("wallet api error %d: %s", res.RetCode, res.RetMsg)
	}
	if len(res.Result.List) == 0 {
		return fmt.Errorf("wallet response has no account")
	}

	balance, err := strconv.ParseFloat(res.Result.List[0].TotalAvailableBalance, 64)
	if err != nil || balance < 0 {
		return fmt.Errorf("invalid totalAvailableBalance")
	}

	e.mu.Lock()
	e.cachedBalance = balance
	e.lastBalanceCheck = time.Now()
	e.mu.Unlock()
	return nil
}

func (e *Engine) MaxScreeningAge() time.Duration {
	return e.cfg.MaxScreeningAge
}

func (e *Engine) ProcessCandidate(ctx context.Context, candidate models.Candidate, targetStrategy string) error {
	side := "Sell"
	if strings.EqualFold(targetStrategy, "long") || strings.EqualFold(targetStrategy, "long-grid") {
		side = "Buy"
	}

	if side != e.targetSide {
		return nil
	}

	result, ok := candidate.Strategies[targetStrategy]
	if !ok || !result.Decision.Eligible {
		return nil
	}

	_ = e.wsEngine.SubscribeTicker(candidate.Symbol)

	e.mu.Lock()
	if e.disabledTokens[candidate.Symbol] {
		e.mu.Unlock()
		return nil
	}
	if _, active := e.positions[candidate.Symbol]; active {
		e.mu.Unlock()
		return nil
	}
	if until, ok := e.cooldowns[candidate.Symbol]; ok {
		if time.Now().Before(until) {
			e.mu.Unlock()
			return nil
		}
		delete(e.cooldowns, candidate.Symbol)
	}

	activeCount := 0
	reservedMargin := 0.0
	for _, pos := range e.positions {
		activeCount++
		reservedMargin += pos.MarginUSD
	}
	if activeCount >= e.cfg.MaxActivePositions {
		e.mu.Unlock()
		return nil
	}
	if reservedMargin+e.cfg.MarginPerTradeUSD > e.cfg.MaxTotalMarginUSD {
		e.mu.Unlock()
		return fmt.Errorf("max total margin would be exceeded")
	}
	if e.cachedBalance < e.cfg.MarginPerTradeUSD {
		e.mu.Unlock()
		return fmt.Errorf("insufficient available balance")
	}

	e.positions[candidate.Symbol] = &models.PositionState{
		Symbol:    candidate.Symbol,
		Side:      side,
		MarginUSD: e.cfg.MarginPerTradeUSD,
		Leverage:  0,
		Managed:   true,
		OpenedAt:  time.Now().UTC(),
		Pending:   true,
	}
	e.candidatesCache[candidate.Symbol] = candidate
	e.mu.Unlock()

	cleanupPending := true
	defer func() {
		if cleanupPending {
			e.mu.Lock()
			if pos, exists := e.positions[candidate.Symbol]; exists && pos.Pending {
				delete(e.positions, candidate.Symbol)
				delete(e.candidatesCache, candidate.Symbol)
				e.cooldowns[candidate.Symbol] = time.Now().Add(3 * time.Minute)
			}
			e.mu.Unlock()
		}
	}()

	bid, ask, _, err := e.getLiveTicker(ctx, candidate.Symbol)
	if err != nil {
		return fmt.Errorf("live ticker: %w", err)
	}
	if bid <= 0 || ask <= bid {
		return fmt.Errorf("invalid live bid/ask")
	}

	qtyStep, minQty, tickSize, minNotional, maxOrderQty, err := e.getInstrumentLimits(ctx, candidate.Symbol)
	if err != nil {
		return fmt.Errorf("instrument limits: %w", err)
	}

	targetLeverage := CalculateDynamicLeverage(candidate, targetStrategy, e.cfg.MaxLeverage)
	e.mu.Lock()
	if pos, ok := e.positions[candidate.Symbol]; ok && pos.Pending {
		pos.Leverage = targetLeverage
	}
	e.mu.Unlock()

	qty, err := CalculatePositionQty(
		e.cfg.MarginPerTradeUSD,
		targetLeverage,
		sideEntryPrice(side, bid, ask),
		qtyStep,
		minQty,
		minNotional,
		maxOrderQty,
	)
	if err != nil {
		return fmt.Errorf("position sizing: %w", err)
	}

	if err := e.setLeverage(ctx, candidate.Symbol, targetLeverage); err != nil {
		return fmt.Errorf("set leverage: %w", err)
	}

	// A PostOnly order must be placed on the maker side of the spread.
	entryPrice := bid
	if side == "Sell" {
		entryPrice = ask
	}
	entryPrice = FormatPriceForSide(entryPrice, side, tickSize)

	sl, tp, _, err := e.calculateRiskLevels(side, entryPrice, candidate, tickSize)
	if err != nil {
		return fmt.Errorf("risk precheck: %w", err)
	}

	orderID, err := e.placePostOnlyOrder(ctx, candidate.Symbol, side, qty, qtyStep, entryPrice, tickSize, sl, tp)
	if err != nil {
		return fmt.Errorf("post-only order: %w", err)
	}

	e.mu.Lock()
	if pos, exists := e.positions[candidate.Symbol]; exists {
		pos.OrderID = orderID
		pos.StopLoss = sl
		pos.TakeProfit = tp
		pos.RiskAttached = true
		if pos.Size <= 0 {
			pos.Pending = true
		}
	}
	e.mu.Unlock()

	cleanupPending = false
	log.Printf("[ORDER PLACED] %s %s qty=%.8f price=%.8f leverage=x%d order=%s",
		candidate.Symbol, side, qty, entryPrice, targetLeverage, orderID)

	return nil
}

func sideEntryPrice(side string, bid, ask float64) float64 {
	if side == "Sell" {
		return ask
	}
	return bid
}

func FormatPriceForSide(price float64, side string, tickSize float64) float64 {
	if tickSize <= 0 {
		return price
	}
	if side == "Buy" {
		return floorToStep(price, tickSize)
	}
	return mathCeilToStep(price, tickSize)
}

func (e *Engine) CleanupPendingOrders(ctx context.Context) {
	timeout := e.cfg.PendingOrderTimeout
	if timeout <= 0 {
		return
	}

	type pending struct {
		symbol  string
		orderID string
		opened  time.Time
	}
	var expired []pending

	e.mu.Lock()
	for symbol, pos := range e.positions {
		if pos.Pending && time.Since(pos.OpenedAt) >= timeout {
			expired = append(expired, pending{symbol: symbol, orderID: pos.OrderID, opened: pos.OpenedAt})
		}
	}
	e.mu.Unlock()

	for _, item := range expired {
		if item.orderID != "" {
			if err := e.cancelOrder(ctx, item.symbol, item.orderID); err != nil {
				log.Printf("[WARN] cancel pending order %s/%s: %v", item.symbol, item.orderID, err)
				continue
			}
		}
		e.mu.Lock()
		if pos, ok := e.positions[item.symbol]; ok && pos.Pending {
			delete(e.positions, item.symbol)
			delete(e.candidatesCache, item.symbol)
			e.cooldowns[item.symbol] = time.Now().Add(3 * time.Minute)
		}
		e.mu.Unlock()
	}
}

func (e *Engine) UpdateTrailingStops(ctx context.Context, symbol string, price float64) {
	if e.cfg.TrailingPct <= 0 || price <= 0 {
		return
	}

	e.mu.Lock()
	pos, ok := e.positions[symbol]
	if !ok || !pos.Managed || pos.Pending || !pos.RiskAttached || pos.StopLoss <= 0 || pos.TakeProfit <= 0 {
		e.mu.Unlock()
		return
	}

	var newSL float64
	if pos.Side == "Buy" {
		if price > pos.HighestPrice {
			pos.HighestPrice = price
		}
		if pos.HighestPrice <= 0 {
			pos.HighestPrice = price
		}
		newSL = pos.HighestPrice * (1 - e.cfg.TrailingPct/100)
		if newSL <= pos.StopLoss {
			e.mu.Unlock()
			return
		}
	} else {
		if pos.LowestPrice == 0 || price < pos.LowestPrice {
			pos.LowestPrice = price
		}
		newSL = pos.LowestPrice * (1 + e.cfg.TrailingPct/100)
		if newSL >= pos.StopLoss {
			e.mu.Unlock()
			return
		}
	}
	oldTP := pos.TakeProfit
	side := pos.Side
	e.mu.Unlock()

	_, _, tickSize, _, _, err := e.getInstrumentLimits(ctx, symbol)
	if err != nil {
		return
	}
	newSL = RoundToStep(newSL, tickSize)
	if err := e.SetTradingStopMarkPrice(ctx, symbol, side, newSL, oldTP, tickSize); err != nil {
		log.Printf("[WARN] trailing stop update %s: %v", symbol, err)
		return
	}

	e.mu.Lock()
	if current, ok := e.positions[symbol]; ok {
		if (current.Side == "Buy" && newSL > current.StopLoss) ||
			(current.Side == "Sell" && newSL < current.StopLoss) {
			current.StopLoss = newSL
		}
	}
	e.mu.Unlock()
}

func (e *Engine) LogActivePositions(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.positions) == 0 {
		log.Printf("[STATE] active positions: 0")
		return
	}
	for symbol, pos := range e.positions {
		log.Printf("[STATE] %s side=%s managed=%v size=%.8f entry=%.8f pending=%v sl=%.8f tp=%.8f",
			symbol, pos.Side, pos.Managed, pos.Size, pos.EntryPrice, pos.Pending, pos.StopLoss, pos.TakeProfit)
	}
}

func (e *Engine) SetTradingStopMarkPrice(ctx context.Context, symbol, side string, sl, tp, tickSize float64) error {
	if sl <= 0 || tp <= 0 {
		return fmt.Errorf("invalid SL/TP")
	}
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"stopLoss":    FormatStep(sl, tickSize),
		"takeProfit":  FormatStep(tp, tickSize),
		"slTriggerBy": "MarkPrice",
		"tpTriggerBy": "MarkPrice",
		"tpslMode":    "Full",
		"positionIdx": 0,
	}
	body, err := e.doSignedPOST(ctx, "/v5/position/trading-stop", params, symbol)
	if err != nil {
		return err
	}
	return checkAPIRetCode(body)
}

func buildPostOnlyOrderParams(symbol, side string, qty, qtyStep, price, tickSize, sl, tp float64) map[string]interface{} {
	return map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        side,
		"orderType":   "Limit",
		"qty":         FormatStep(qty, qtyStep),
		"price":       FormatStep(price, tickSize),
		"timeInForce": "PostOnly",
		"positionIdx": 0,
		"reduceOnly":  false,
		"takeProfit":  FormatStep(tp, tickSize),
		"stopLoss":    FormatStep(sl, tickSize),
		"tpTriggerBy": "MarkPrice",
		"slTriggerBy": "MarkPrice",
		"tpslMode":    "Full",
		"tpOrderType": "Market",
		"slOrderType": "Market",
	}
}

func (e *Engine) placePostOnlyOrder(ctx context.Context, symbol, side string, qty, qtyStep, price, tickSize, sl, tp float64) (string, error) {
	params := buildPostOnlyOrderParams(symbol, side, qty, qtyStep, price, tickSize, sl, tp)
	body, err := e.doSignedPOST(ctx, "/v5/order/create", params, symbol)
	if err != nil {
		return "", err
	}

	var res struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			OrderID string `json:"orderId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if res.RetCode != 0 {
		return "", fmt.Errorf("bybit api %d: %s", res.RetCode, res.RetMsg)
	}
	if res.Result.OrderID == "" {
		return "", fmt.Errorf("empty order id")
	}
	return res.Result.OrderID, nil
}

func (e *Engine) cancelOrder(ctx context.Context, symbol, orderID string) error {
	body, err := e.doSignedPOST(ctx, "/v5/order/cancel", map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
		"orderId":  orderID,
	}, symbol)
	if err != nil {
		return err
	}
	return checkAPIRetCode(body)
}

func (e *Engine) setLeverage(ctx context.Context, symbol string, leverage int) error {
	lev := strconv.Itoa(leverage)
	body, err := e.doSignedPOST(ctx, "/v5/position/set-leverage", map[string]interface{}{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  lev,
		"sellLeverage": lev,
	}, symbol)
	if err != nil {
		return err
	}

	var res struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}
	if res.RetCode != 0 && res.RetCode != 110043 {
		return fmt.Errorf("set leverage api %d: %s", res.RetCode, res.RetMsg)
	}
	return nil
}

func (e *Engine) refreshPositions(ctx context.Context) error {
	body, err := e.doSignedGET(ctx, "/v5/position/list", url.Values{
		"category":   {"linear"},
		"settleCoin": {"USDT"},
	}.Encode())
	if err != nil {
		return err
	}

	var res struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				Symbol     string `json:"symbol"`
				Side       string `json:"side"`
				Size       string `json:"size"`
				EntryPrice string `json:"entryPrice"`
				StopLoss   string `json:"stopLoss"`
				TakeProfit string `json:"takeProfit"`
				Leverage   string `json:"leverage"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}
	if res.RetCode != 0 {
		return fmt.Errorf("position api %d: %s", res.RetCode, res.RetMsg)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, item := range res.Result.List {
		size, _ := strconv.ParseFloat(item.Size, 64)
		if size <= 0 || item.Side == "" {
			continue
		}
		entry, _ := strconv.ParseFloat(item.EntryPrice, 64)
		sl, _ := strconv.ParseFloat(item.StopLoss, 64)
		tp, _ := strconv.ParseFloat(item.TakeProfit, 64)
		leverage, _ := strconv.ParseFloat(item.Leverage, 64)
		margin := 0.0
		if leverage > 0 {
			margin = math.Abs(size*entry) / leverage
		}
		if margin <= 0 {
			margin = e.cfg.MarginPerTradeUSD
		}
		e.positions[item.Symbol] = &models.PositionState{
			Symbol:       item.Symbol,
			Side:         item.Side,
			EntryPrice:   entry,
			Size:         size,
			StopLoss:     sl,
			TakeProfit:   tp,
			MarginUSD:    margin,
			Leverage:     int(leverage),
			OpenedAt:     time.Now().UTC(),
			Managed:      false,
			RiskAttached: sl > 0 && tp > 0,
		}
	}
	return nil
}

func (e *Engine) refreshOpenOrders(ctx context.Context) error {
	body, err := e.doSignedGET(ctx, "/v5/order/realtime", url.Values{
		"category":   {"linear"},
		"settleCoin": {"USDT"},
		"limit":      {"50"},
	}.Encode())
	if err != nil {
		return err
	}

	var res struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				Symbol       string `json:"symbol"`
				OrderID      string `json:"orderId"`
				Side         string `json:"side"`
				OrderStatus  string `json:"orderStatus"`
				ReduceOnly   bool   `json:"reduceOnly"`
				TriggerPrice string `json:"triggerPrice"`
				CumExecQty   string `json:"cumExecQty"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}
	if res.RetCode != 0 {
		return fmt.Errorf("open orders api %d: %s", res.RetCode, res.RetMsg)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, item := range res.Result.List {
		if item.ReduceOnly || item.TriggerPrice != "" && item.TriggerPrice != "0" && item.TriggerPrice != "0.00" {
			continue
		}
		if item.OrderStatus != "New" && item.OrderStatus != "PartiallyFilled" {
			continue
		}
		qty, _ := strconv.ParseFloat(item.CumExecQty, 64)
		if _, exists := e.positions[item.Symbol]; !exists {
			e.positions[item.Symbol] = &models.PositionState{
				Symbol:    item.Symbol,
				Side:      item.Side,
				OrderID:   item.OrderID,
				OpenedAt:  time.Now().UTC(),
				MarginUSD: e.cfg.MarginPerTradeUSD,
				Managed:   false,
				Pending:   true,
			}
		} else if e.positions[item.Symbol].Pending {
			e.positions[item.Symbol].OrderID = item.OrderID
			e.positions[item.Symbol].Pending = true
			_ = qty
		}
	}
	return nil
}

func (e *Engine) getLiveTicker(ctx context.Context, symbol string) (bid, ask, last float64, err error) {
	body, err := e.publicGET(ctx, "/v5/market/tickers", url.Values{
		"category": {"linear"},
		"symbol":   {symbol},
	}.Encode())
	if err != nil {
		return 0, 0, 0, err
	}

	var res struct {
		Result struct {
			List []struct {
				Bid  string `json:"bid1Price"`
				Ask  string `json:"ask1Price"`
				Last string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Result.List) == 0 {
		return 0, 0, 0, fmt.Errorf("ticker parse error")
	}
	item := res.Result.List[0]
	bid, _ = strconv.ParseFloat(item.Bid, 64)
	ask, _ = strconv.ParseFloat(item.Ask, 64)
	last, _ = strconv.ParseFloat(item.Last, 64)
	if bid <= 0 || ask <= bid {
		return 0, 0, 0, fmt.Errorf("invalid ticker bid/ask")
	}
	return bid, ask, last, nil
}

func (e *Engine) getInstrumentLimits(ctx context.Context, symbol string) (qtyStep, minQty, tickSize, minNotional, maxOrderQty float64, err error) {
	body, err := e.publicGET(ctx, "/v5/market/instruments-info", url.Values{
		"category": {"linear"},
		"symbol":   {symbol},
	}.Encode())
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	var res struct {
		Result struct {
			List []struct {
				PriceFilter struct {
					TickSize string `json:"tickSize"`
				} `json:"priceFilter"`
				LotSizeFilter struct {
					QtyStep     string `json:"qtyStep"`
					MinOrderQty string `json:"minOrderQty"`
					MinNotional string `json:"minNotionalValue"`
					MaxOrderQty string `json:"maxOrderQty"`
				} `json:"lotSizeFilter"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Result.List) == 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("instrument parse error")
	}
	item := res.Result.List[0]
	return parseFloat(item.LotSizeFilter.QtyStep),
		parseFloat(item.LotSizeFilter.MinOrderQty),
		parseFloat(item.PriceFilter.TickSize),
		parseFloat(item.LotSizeFilter.MinNotional),
		parseFloat(item.LotSizeFilter.MaxOrderQty),
		nil
}

func (e *Engine) publicGET(ctx context.Context, path, query string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+path+"?"+query, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	if err := checkAPIRetCode(body); err != nil {
		return nil, err
	}
	return body, nil
}

func (e *Engine) doSignedGET(ctx context.Context, path, query string) ([]byte, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"
	raw := timestamp + e.cfg.ApiKey + recvWindow + query

	h := hmac.New(sha256.New, []byte(e.cfg.ApiSecret))
	_, _ = h.Write([]byte(raw))
	signature := hex.EncodeToString(h.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+path+"?"+query, nil)
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	if err := checkAPIRetCode(body); err != nil {
		return nil, err
	}
	return body, nil
}

func (e *Engine) doSignedPOST(ctx context.Context, path string, payload map[string]interface{}, symbol string) ([]byte, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"
	raw := timestamp + e.cfg.ApiKey + recvWindow + string(jsonBody)

	h := hmac.New(sha256.New, []byte(e.cfg.ApiSecret))
	_, _ = h.Write([]byte(raw))
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var apiRes struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if json.Unmarshal(body, &apiRes) == nil && apiRes.RetCode == 110126 {
		e.mu.Lock()
		e.disabledTokens[symbol] = true
		e.mu.Unlock()
	}
	return body, nil
}

func checkAPIRetCode(body []byte) error {
	var res struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("Bybit response parse error: %w", err)
	}
	if res.RetCode != 0 {
		return fmt.Errorf("Bybit API error %d: %s", res.RetCode, res.RetMsg)
	}
	return nil
}

func parseFloat(value string) float64 {
	v, _ := strconv.ParseFloat(value, 64)
	return v
}

func mathCeilToStep(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	return math.Ceil(value/step) * step
}

func mathAbsPct(distance, base float64) float64 {
	if base <= 0 {
		return 0
	}
	return distance / base * 100
}
