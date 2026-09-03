// internal/execution/ws_engine.go
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
}

type WSEngine struct {
	apiKey          string
	apiSecret       string
	testnet         bool
	onPosClosed     func(symbol string)
	onBalanceUpdate func(balance float64)
	onExecution     func(exec ExecutionLog)

	publicConn  *websocket.Conn
	privateConn *websocket.Conn

	mu         sync.RWMutex
	prices     map[string]float64
	subscribed map[string]bool
	pubConnMu  sync.Mutex
	privConnMu sync.Mutex
}

func NewWSEngine(
	apiKey, apiSecret string,
	testnet bool,
	onPosClosed func(string),
	onBalanceUpdate func(float64),
	onExecution func(ExecutionLog),
) *WSEngine {
	return &WSEngine{
		apiKey:          apiKey,
		apiSecret:       apiSecret,
		testnet:         testnet,
		onPosClosed:     onPosClosed,
		onBalanceUpdate: onBalanceUpdate,
		onExecution:     onExecution,
		prices:          make(map[string]float64),
		subscribed:      make(map[string]bool),
	}
}

func (w *WSEngine) GetLatestPrice(symbol string) (float64, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	price, exists := w.prices[symbol]
	return price, exists && price > 0
}

func (w *WSEngine) StartPublicTickerStream(ctx context.Context) error {
	url := "wss://stream.bybit.com/v5/public/linear"
	if w.testnet {
		url = "wss://stream-testnet.bybit.com/v5/public/linear"
	}

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

		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("[WS WARN] Public Dial error: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		w.pubConnMu.Lock()
		w.publicConn = conn
		w.pubConnMu.Unlock()

		w.resubscribeTickers()

		pingCtx, pingCancel := context.WithCancel(ctx)
		go w.startHeartbeat(pingCtx, conn, &w.pubConnMu, "Public")

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS WARN] Public Read error: %v. Reconnecting...", err)
				pingCancel()
				_ = conn.Close()
				break
			}

			var msg struct {
				Topic string `json:"topic"`
				Data  struct {
					Symbol    string `json:"symbol"`
					LastPrice string `json:"lastPrice"`
				} `json:"data"`
			}

			if err := json.Unmarshal(message, &msg); err == nil && msg.Topic != "" {
				if price, err := strconv.ParseFloat(msg.Data.LastPrice, 64); err == nil && price > 0 {
					w.mu.Lock()
					w.prices[msg.Data.Symbol] = price
					w.mu.Unlock()
				}
			}
		}
		pingCancel()
		time.Sleep(2 * time.Second)
	}
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

	req := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{fmt.Sprintf("tickers.%s", symbol)},
	}
	return w.publicConn.WriteJSON(req)
}

func (w *WSEngine) resubscribeTickers() {
	w.mu.RLock()
	symbols := make([]string, 0, len(w.subscribed))
	for s := range w.subscribed {
		symbols = append(symbols, fmt.Sprintf("tickers.%s", s))
	}
	w.mu.RUnlock()

	if len(symbols) == 0 {
		return
	}

	w.pubConnMu.Lock()
	defer w.pubConnMu.Unlock()

	if w.publicConn != nil {
		req := map[string]interface{}{
			"op":   "subscribe",
			"args": symbols,
		}
		_ = w.publicConn.WriteJSON(req)
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

		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("[WS WARN] Private Dial error: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		expires := time.Now().UnixMilli() + 10000
		val := fmt.Sprintf("GET/realtime%d", expires)
		h := hmac.New(sha256.New, []byte(w.apiSecret))
		h.Write([]byte(val))
		sig := hex.EncodeToString(h.Sum(nil))

		authReq := map[string]interface{}{
			"op":   "auth",
			"args": []interface{}{w.apiKey, expires, sig},
		}

		if err := conn.WriteJSON(authReq); err != nil {
			log.Printf("[WS ERROR] Private auth payload send failed: %v", err)
			_ = conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}
		log.Printf("[WS INFO] Private auth request dispatched successfully.")

		w.privConnMu.Lock()
		w.privateConn = conn
		w.privConnMu.Unlock()

		subReq := map[string]interface{}{
			"op":   "subscribe",
			"args": []string{"position", "wallet", "execution"},
		}
		if err := conn.WriteJSON(subReq); err != nil {
			log.Printf("[WS ERROR] Private subscribe payload send failed: %v", err)
		} else {
			log.Printf("[WS INFO] Subscribed to private topics: [position, wallet, execution].")
		}

		pingCtx, pingCancel := context.WithCancel(ctx)
		go w.startHeartbeat(pingCtx, conn, &w.privConnMu, "Private")

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS WARN] Private Read error: %v. Reconnecting...", err)
				pingCancel()
				_ = conn.Close()
				break
			}

			w.parsePrivateMessage(message)
		}
		pingCancel()
		time.Sleep(2 * time.Second)
	}
}

func (w *WSEngine) startHeartbeat(ctx context.Context, conn *websocket.Conn, mu *sync.Mutex, connType string) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			if conn != nil {
				pingReq := map[string]string{"op": "ping"}
				if err := conn.WriteJSON(pingReq); err != nil {
					log.Printf("[WS WARN] %s Ping failed: %v", connType, err)
				}
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

	if err := json.Unmarshal(message, &base); err != nil || base.Topic == "" {
		return
	}

	switch base.Topic {
	case "position":
		var posData []struct {
			Symbol string `json:"symbol"`
			Size   string `json:"size"`
		}
		if err := json.Unmarshal(base.Data, &posData); err == nil {
			for _, pos := range posData {
				if size, err := strconv.ParseFloat(pos.Size, 64); err == nil && size == 0 {
					if w.onPosClosed != nil {
						w.onPosClosed(pos.Symbol)
					}
				}
			}
		}

	case "wallet":
		var walletData []struct {
			Coin []struct {
				Coin                string `json:"coin"`
				AvailableToWithdraw string `json:"availableToWithdraw"`
				WalletBalance       string `json:"walletBalance"`
			} `json:"coin"`
		}
		if err := json.Unmarshal(base.Data, &walletData); err == nil {
			for _, item := range walletData {
				for _, coin := range item.Coin {
					if coin.Coin == "USDT" {
						balStr := coin.AvailableToWithdraw
						if balStr == "" {
							balStr = coin.WalletBalance
						}
						if bal, err := strconv.ParseFloat(balStr, 64); err == nil && bal >= 0 {
							if w.onBalanceUpdate != nil {
								w.onBalanceUpdate(bal)
							}
						}
					}
				}
			}
		}

	case "execution":
		var execData []struct {
			Symbol     string `json:"symbol"`
			Side       string `json:"side"`
			ExecPrice  string `json:"execPrice"`
			ExecQty    string `json:"execQty"`
			ExecFee    string `json:"execFee"`
			OrderType  string `json:"orderType"`
			ExecType   string `json:"execType"`
			ClosedSize string `json:"closedSize"`
			ExecTime   string `json:"execTime"`
		}
		if err := json.Unmarshal(base.Data, &execData); err == nil {
			for _, exec := range execData {
				closedSz, _ := strconv.ParseFloat(exec.ClosedSize, 64)
				if closedSz > 0 && w.onExecution != nil {
					price, _ := strconv.ParseFloat(exec.ExecPrice, 64)
					qty, _ := strconv.ParseFloat(exec.ExecQty, 64)
					fee, _ := strconv.ParseFloat(exec.ExecFee, 64)
					tsMS, _ := strconv.ParseInt(exec.ExecTime, 10, 64)

					w.onExecution(ExecutionLog{
						Symbol:     exec.Symbol,
						Side:       exec.Side,
						ExecPrice:  price,
						ExecQty:    qty,
						ExecFee:    fee,
						OrderType:  exec.OrderType,
						ExecType:   exec.ExecType,
						ClosedSize: closedSz,
						ExecTime:   time.UnixMilli(tsMS).UTC(),
					})
				}
			}
		}
	}
}
