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
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSEngine struct {
	apiKey      string
	apiSecret   string
	testnet     bool
	mu          sync.RWMutex
	prices      map[string]float64
	subscribed  map[string]bool
	pubConn     *websocket.Conn
	privConn    *websocket.Conn
	pubConnMu   sync.Mutex
	onClosedPos func(symbol string)
}

func NewWSEngine(apiKey, apiSecret string, testnet bool, onClosedPos func(symbol string)) *WSEngine {
	return &WSEngine{
		apiKey:      apiKey,
		apiSecret:   apiSecret,
		testnet:     testnet,
		prices:      make(map[string]float64),
		subscribed:  make(map[string]bool),
		onClosedPos: onClosedPos,
	}
}

func (ws *WSEngine) StartPublicTickerStream(ctx context.Context) error {
	wsURL := "wss://stream.bybit.com/v5/public/linear"
	if ws.testnet {
		wsURL = "wss://stream-testnet.bybit.com/v5/public/linear"
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("public ws dial failed: %w", err)
	}

	ws.pubConnMu.Lock()
	ws.pubConn = conn
	ws.pubConnMu.Unlock()

	go ws.readPublicLoop(ctx)
	return nil
}

func (ws *WSEngine) SubscribeTicker(symbol string) error {
	ws.mu.Lock()
	if ws.subscribed[symbol] {
		ws.mu.Unlock()
		return nil
	}
	ws.subscribed[symbol] = true
	ws.mu.Unlock()

	ws.pubConnMu.Lock()
	defer ws.pubConnMu.Unlock()

	if ws.pubConn == nil {
		return fmt.Errorf("public websocket connection is nil")
	}

	subPayload := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{fmt.Sprintf("tickers.%s", symbol)},
	}

	return ws.pubConn.WriteJSON(subPayload)
}

func (ws *WSEngine) readPublicLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			ws.pubConnMu.Lock()
			if ws.pubConn != nil {
				_ = ws.pubConn.Close()
			}
			ws.pubConnMu.Unlock()
			return
		default:
			ws.pubConnMu.Lock()
			conn := ws.pubConn
			ws.pubConnMu.Unlock()

			if conn == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[WS ERROR] Public read failed: %v. Reconnecting...", err)
				time.Sleep(2 * time.Second)
				_ = ws.StartPublicTickerStream(ctx)
				return
			}

			var msg struct {
				Topic string `json:"topic"`
				Data  struct {
					Symbol    string `json:"symbol"`
					LastPrice string `json:"lastPrice"`
				} `json:"data"`
			}

			if err := json.Unmarshal(message, &msg); err == nil && msg.Data.LastPrice != "" {
				price, err := strconv.ParseFloat(msg.Data.LastPrice, 64)
				if err == nil && price > 0 {
					ws.mu.Lock()
					ws.prices[msg.Data.Symbol] = price
					ws.mu.Unlock()
				}
			}
		}
	}
}

func (ws *WSEngine) StartPrivateStream(ctx context.Context) error {
	wsURL := "wss://stream.bybit.com/v5/private"
	if ws.testnet {
		wsURL = "wss://stream-testnet.bybit.com/v5/private"
	}

	u, err := url.Parse(wsURL)
	if err != nil {
		return err
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("private ws dial failed: %w", err)
	}
	ws.privConn = conn

	expires := time.Now().UnixMilli() + 10000
	sig := generatePrivateWSSignature(ws.apiKey, ws.apiSecret, expires)

	authPayload := map[string]interface{}{
		"op":   "auth",
		"args": []interface{}{ws.apiKey, expires, sig},
	}

	if err := conn.WriteJSON(authPayload); err != nil {
		_ = conn.Close()
		return fmt.Errorf("private ws auth send failed: %w", err)
	}

	subPayload := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"position"},
	}

	if err := conn.WriteJSON(subPayload); err != nil {
		_ = conn.Close()
		return fmt.Errorf("private ws subscribe failed: %w", err)
	}

	go ws.readPrivateLoop(ctx)
	return nil
}

func (ws *WSEngine) readPrivateLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			if ws.privConn != nil {
				_ = ws.privConn.Close()
			}
			return
		default:
			if ws.privConn == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			_, message, err := ws.privConn.ReadMessage()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[WS ERROR] Private read failed: %v. Reconnecting...", err)
				time.Sleep(2 * time.Second)
				_ = ws.StartPrivateStream(ctx)
				return
			}

			var msg struct {
				Topic string `json:"topic"`
				Data  []struct {
					Symbol string `json:"symbol"`
					Size   string `json:"size"`
				} `json:"data"`
			}

			if err := json.Unmarshal(message, &msg); err == nil && msg.Topic == "position" {
				for _, pos := range msg.Data {
					size, err := strconv.ParseFloat(pos.Size, 64)
					if err == nil && size == 0 {
						if ws.onClosedPos != nil {
							ws.onClosedPos(pos.Symbol)
						}
					}
				}
			}
		}
	}
}

func (ws *WSEngine) GetLatestPrice(symbol string) (float64, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	price, exists := ws.prices[symbol]
	return price, exists
}

func generatePrivateWSSignature(apiKey, apiSecret string, expires int64) string {
	val := fmt.Sprintf("GET/realtime%d", expires)
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(val))
	return hex.EncodeToString(h.Sum(nil))
}
