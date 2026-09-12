package execution

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ExecutionLog struct {
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	ExecPrice  float64   `json:"execPrice"`
	ExecQty    float64   `json:"execQty"`
	ExecFee    float64   `json:"execFee"`
	OrderType  string    `json:"orderType"`
	ExecType   string    `json:"execType"`
	ClosedSize float64   `json:"closedSize"`
	ExecTime   time.Time `json:"execTime"`
	ExecID     string    `json:"execId"`
	OrderID    string    `json:"orderId"`
}

type PositionUpdate struct {
	Symbol     string
	Side       string
	Size       float64
	EntryPrice float64
}

type OrderUpdate struct {
	Symbol     string
	OrderID    string
	Side       string
	Status     string
	CumExecQty float64
}

type WSEngine struct {
	apiKey    string
	apiSecret string
	testnet   bool

	onPosition      func(PositionUpdate)
	onOrder         func(OrderUpdate)
	onExecution     func(ExecutionLog)
	onBalanceUpdate func(float64)

	publicConn  *websocket.Conn
	privateConn *websocket.Conn

	mu         sync.RWMutex
	prices     map[string]float64
	subscribed map[string]bool
	pubConnMu  sync.Mutex
	privConnMu sync.Mutex

	btcBase15mPrice float64
	lastBTCReset    time.Time
}

func NewWSEngine(
	apiKey, apiSecret string,
	testnet bool,
	onPosition func(PositionUpdate),
	onOrder func(OrderUpdate),
	onExecution func(ExecutionLog),
	onBalanceUpdate func(float64),
) *WSEngine {
	return &WSEngine{
		apiKey:          apiKey,
		apiSecret:       apiSecret,
		testnet:         testnet,
		onPosition:      onPosition,
		onOrder:         onOrder,
		onExecution:     onExecution,
		onBalanceUpdate: onBalanceUpdate,
		prices:          make(map[string]float64),
		subscribed:      make(map[string]bool),
		lastBTCReset:    time.Now(),
	}
}

func (w *WSEngine) GetLatestPrice(symbol string) (float64, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	price, ok := w.prices[symbol]
	return price, ok && price > 0
}

func (w *WSEngine) GetBTCTrend15m() (float64, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	price, ok := w.prices["BTCUSDT"]
	if !ok || price <= 0 || w.btcBase15mPrice <= 0 {
		return 0, false
	}
	return (price - w.btcBase15mPrice) / w.btcBase15mPrice * 100, true
}

func (w *WSEngine) StartPublicTickerStream(ctx context.Context) error {
	url := "wss://stream.bybit.com/v5/public/linear"
	if w.testnet {
		url = "wss://stream-testnet.bybit.com/v5/public/linear"
	}
	w.mu.Lock()
	w.subscribed["BTCUSDT"] = true
	w.mu.Unlock()
	go w.connectAndReadPublic(ctx, url)
	return nil
}

func (w *WSEngine) connectAndReadPublic(ctx context.Context, url string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			log.Printf("[WS WARN] public dial: %v", err)
			if !sleepContext(ctx, 5*time.Second) {
				return
			}
			continue
		}

		w.pubConnMu.Lock()
		w.publicConn = conn
		w.resubscribeTickersLocked()
		w.pubConnMu.Unlock()

		pingCtx, cancel := context.WithCancel(ctx)
		go w.startHeartbeat(pingCtx, conn, &w.pubConnMu, "Public")

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				cancel()
				_ = conn.Close()
				break
			}
			w.parsePublicMessage(message)
		}
		cancel()

		w.pubConnMu.Lock()
		if w.publicConn == conn {
			w.publicConn = nil
		}
		w.pubConnMu.Unlock()

		if !sleepContext(ctx, 2*time.Second) {
			return
		}
	}
}

func (w *WSEngine) parsePublicMessage(message []byte) {
	var msg struct {
		Topic string          `json:"topic"`
		Data  json.RawMessage `json:"data"`
	}
	if json.Unmarshal(message, &msg) != nil || msg.Topic == "" {
		return
	}

	var data struct {
		Symbol    string `json:"symbol"`
		LastPrice string `json:"lastPrice"`
	}
	if json.Unmarshal(msg.Data, &data) != nil || data.Symbol == "" {
		return
	}
	price, err := strconv.ParseFloat(data.LastPrice, 64)
	if err != nil || price <= 0 {
		return
	}

	w.mu.Lock()
	w.prices[data.Symbol] = price
	if data.Symbol == "BTCUSDT" &&
		(w.btcBase15mPrice == 0 || time.Since(w.lastBTCReset) >= 15*time.Minute) {
		w.btcBase15mPrice = price
		w.lastBTCReset = time.Now()
	}
	w.mu.Unlock()
}

func (w *WSEngine) SubscribeTicker(symbol string) error {
	w.mu.Lock()
	if w.subscribed[symbol] {
		w.mu.Unlock()
		return nil
	}
	w.subscribed[symbol] = true
	w.mu.Unlock()

	w.pubConnMu.Lock()
	defer w.pubConnMu.Unlock()
	if w.publicConn == nil {
		return nil
	}
	return w.publicConn.WriteJSON(map[string]interface{}{
		"op":   "subscribe",
		"args": []string{fmt.Sprintf("tickers.%s", symbol)},
	})
}

func (w *WSEngine) resubscribeTickersLocked() {
	w.mu.RLock()
	args := make([]string, 0, len(w.subscribed))
	for symbol := range w.subscribed {
		args = append(args, fmt.Sprintf("tickers.%s", symbol))
	}
	w.mu.RUnlock()
	if w.publicConn != nil && len(args) > 0 {
		_ = w.publicConn.WriteJSON(map[string]interface{}{"op": "subscribe", "args": args})
	}
}

func (w *WSEngine) StartPrivateStream(ctx context.Context) error {
	url := "wss://stream.bybit.com/v5/private"
	if w.testnet {
		url = "wss://stream-testnet.bybit.com/v5/private"
	}
	go w.connectAndReadPrivate(ctx, url)
	return nil
}

func (w *WSEngine) connectAndReadPrivate(ctx context.Context, url string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			log.Printf("[WS WARN] private dial: %v", err)
			if !sleepContext(ctx, 5*time.Second) {
				return
			}
			continue
		}

		expires := time.Now().UnixMilli() + 10000
		message := fmt.Sprintf("GET/realtime%d", expires)
		h := hmac.New(sha256.New, []byte(w.apiSecret))
		_, _ = h.Write([]byte(message))
		signature := hex.EncodeToString(h.Sum(nil))

		if err := conn.WriteJSON(map[string]interface{}{
			"op":   "auth",
			"args": []interface{}{w.apiKey, expires, signature},
		}); err != nil {
			_ = conn.Close()
			if !sleepContext(ctx, 3*time.Second) {
				return
			}
			continue
		}

		w.privConnMu.Lock()
		w.privateConn = conn
		w.privConnMu.Unlock()

		if err := conn.WriteJSON(map[string]interface{}{
			"op":   "subscribe",
			"args": []string{"position", "wallet", "execution", "order"},
		}); err != nil {
			log.Printf("[WS WARN] private subscribe: %v", err)
		}

		pingCtx, cancel := context.WithCancel(ctx)
		go w.startHeartbeat(pingCtx, conn, &w.privConnMu, "Private")

		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				cancel()
				_ = conn.Close()
				break
			}
			w.parsePrivateMessage(raw)
		}
		cancel()

		w.privConnMu.Lock()
		if w.privateConn == conn {
			w.privateConn = nil
		}
		w.privConnMu.Unlock()

		if !sleepContext(ctx, 2*time.Second) {
			return
		}
	}
}

func (w *WSEngine) startHeartbeat(ctx context.Context, conn *websocket.Conn, mu *sync.Mutex, name string) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			if err := conn.WriteJSON(map[string]string{"op": "ping"}); err != nil {
				log.Printf("[WS WARN] %s heartbeat: %v", name, err)
			}
			mu.Unlock()
		}
	}
}

func (w *WSEngine) parsePrivateMessage(message []byte) {
	var base struct {
		Topic string          `json:"topic"`
		Data  json.RawMessage `json:"data"`
	}
	if json.Unmarshal(message, &base) != nil || base.Topic == "" {
		return
	}

	switch base.Topic {
	case "position":
		var data []struct {
			Symbol     string `json:"symbol"`
			Side       string `json:"side"`
			Size       string `json:"size"`
			EntryPrice string `json:"entryPrice"`
		}
		if json.Unmarshal(base.Data, &data) != nil {
			return
		}
		for _, item := range data {
			size, _ := strconv.ParseFloat(item.Size, 64)
			entry, _ := strconv.ParseFloat(item.EntryPrice, 64)
			if w.onPosition != nil {
				w.onPosition(PositionUpdate{
					Symbol:     item.Symbol,
					Side:       item.Side,
					Size:       size,
					EntryPrice: entry,
				})
			}
		}

	case "wallet":
		var data []struct {
			TotalAvailableBalance string `json:"totalAvailableBalance"`
		}
		if json.Unmarshal(base.Data, &data) == nil && len(data) > 0 {
			if balance, err := strconv.ParseFloat(data[0].TotalAvailableBalance, 64); err == nil && balance >= 0 {
				if w.onBalanceUpdate != nil {
					w.onBalanceUpdate(balance)
				}
			}
		}

	case "order":
		var data []struct {
			Symbol      string `json:"symbol"`
			OrderID     string `json:"orderId"`
			Side        string `json:"side"`
			OrderStatus string `json:"orderStatus"`
			CumExecQty  string `json:"cumExecQty"`
		}
		if json.Unmarshal(base.Data, &data) != nil {
			return
		}
		for _, item := range data {
			qty, _ := strconv.ParseFloat(item.CumExecQty, 64)
			if w.onOrder != nil {
				w.onOrder(OrderUpdate{
					Symbol:     item.Symbol,
					OrderID:    item.OrderID,
					Side:       item.Side,
					Status:     item.OrderStatus,
					CumExecQty: qty,
				})
			}
		}

	case "execution":
		var data []struct {
			Symbol     string `json:"symbol"`
			Side       string `json:"side"`
			ExecPrice  string `json:"execPrice"`
			ExecQty    string `json:"execQty"`
			ExecFee    string `json:"execFee"`
			OrderType  string `json:"orderType"`
			ExecType   string `json:"execType"`
			ClosedSize string `json:"closedSize"`
			ExecTime   string `json:"execTime"`
			ExecID     string `json:"execId"`
			OrderID    string `json:"orderId"`
		}
		if json.Unmarshal(base.Data, &data) != nil {
			return
		}
		for _, item := range data {
			price, _ := strconv.ParseFloat(item.ExecPrice, 64)
			qty, _ := strconv.ParseFloat(item.ExecQty, 64)
			fee, _ := strconv.ParseFloat(item.ExecFee, 64)
			closed, _ := strconv.ParseFloat(item.ClosedSize, 64)
			ms, _ := strconv.ParseInt(item.ExecTime, 10, 64)
			if w.onExecution != nil && (qty > 0 || closed > 0) {
				w.onExecution(ExecutionLog{
					Symbol:     item.Symbol,
					Side:       item.Side,
					ExecPrice:  price,
					ExecQty:    qty,
					ExecFee:    fee,
					OrderType:  item.OrderType,
					ExecType:   item.ExecType,
					ClosedSize: closed,
					ExecTime:   time.UnixMilli(ms).UTC(),
					ExecID:     item.ExecID,
					OrderID:    item.OrderID,
				})
			}
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
