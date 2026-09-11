package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultWsURL = "wss://stream.bybit.com/v5/public/linear"

type PublicWSStream struct {
	obCache        *OrderBookCache
	klineCache     *KlineCache
	conn           *websocket.Conn
	mu             sync.Mutex
	subTopics      map[string]bool
	isCtxDone      bool
	started        bool
	orderBookDepth int
}

func NewPublicWSStream(ob *OrderBookCache, kc *KlineCache, orderBookDepth int) *PublicWSStream {
	if orderBookDepth != 50 && orderBookDepth != 200 && orderBookDepth != 1000 {
		orderBookDepth = 50
	}
	return &PublicWSStream{
		obCache:        ob,
		klineCache:     kc,
		subTopics:      make(map[string]bool),
		orderBookDepth: orderBookDepth,
	}
}

func (ws *PublicWSStream) topicsForSymbol(symbol string) []string {
	return []string{
		fmt.Sprintf("orderbook.%d.%s", ws.orderBookDepth, symbol),
		fmt.Sprintf("kline.5.%s", symbol),
		fmt.Sprintf("kline.15.%s", symbol),
		fmt.Sprintf("kline.30.%s", symbol),
		fmt.Sprintf("kline.60.%s", symbol),
		fmt.Sprintf("kline.240.%s", symbol),
	}
}

func (ws *PublicWSStream) AddSymbols(symbols []string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	for _, symbol := range symbols {
		for _, topic := range ws.topicsForSymbol(symbol) {
			ws.subTopics[topic] = true
		}
	}
	if ws.conn != nil {
		ws.subscribeAllLocked()
	}
}

func (ws *PublicWSStream) Start(ctx context.Context, symbols []string) error {
	ws.AddSymbols(symbols)

	ws.mu.Lock()
	if ws.started {
		ws.mu.Unlock()
		return nil
	}
	ws.started = true
	ws.mu.Unlock()

	go ws.loop(ctx)
	return nil
}

func (ws *PublicWSStream) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			ws.mu.Lock()
			ws.isCtxDone = true
			if ws.conn != nil {
				_ = ws.conn.Close()
			}
			ws.mu.Unlock()
			return
		default:
		}

		dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
		conn, _, err := dialer.DialContext(ctx, defaultWsURL, http.Header{})
		if err != nil {
			log.Printf("[WS WARN] Public stream dial error: %v. Retrying in 5s...", err)
			if !sleepContext(ctx, 5*time.Second) {
				return
			}
			continue
		}

		ws.mu.Lock()
		ws.conn = conn
		ws.isCtxDone = false
		ws.subscribeAllLocked()
		ws.mu.Unlock()

		pingCtx, pingCancel := context.WithCancel(ctx)
		go ws.keepAlive(pingCtx, conn)

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				pingCancel()
				ws.mu.Lock()
				if !ws.isCtxDone {
					log.Printf("[WS WARN] Public stream disconnect: %v. Reconnecting...", err)
				}
				ws.mu.Unlock()
				_ = conn.Close()
				break
			}
			ws.parseMessage(msgBytes)
		}

		pingCancel()
		ws.mu.Lock()
		if ws.conn == conn {
			ws.conn = nil
		}
		ws.mu.Unlock()

		if !sleepContext(ctx, 2*time.Second) {
			return
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (ws *PublicWSStream) subscribeAllLocked() {
	if ws.conn == nil || len(ws.subTopics) == 0 {
		return
	}

	topics := make([]string, 0, len(ws.subTopics))
	for topic := range ws.subTopics {
		topics = append(topics, topic)
	}

	const batchSize = 50
	for i := 0; i < len(topics); i += batchSize {
		end := i + batchSize
		if end > len(topics) {
			end = len(topics)
		}
		msg := map[string]interface{}{
			"op":   "subscribe",
			"args": topics[i:end],
		}
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Printf("[WS WARN] subscription failed: %v", err)
		}
	}
}

func (ws *PublicWSStream) keepAlive(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ws.mu.Lock()
			if ws.conn == conn {
				_ = conn.WriteJSON(map[string]string{"op": "ping"})
			}
			ws.mu.Unlock()
		}
	}
}

func (ws *PublicWSStream) parseMessage(msgBytes []byte) {
	var base struct {
		Topic string          `json:"topic"`
		Type  string          `json:"type"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msgBytes, &base); err != nil || base.Topic == "" {
		return
	}

	if strings.HasPrefix(base.Topic, "orderbook.") {
		parts := strings.Split(base.Topic, ".")
		if len(parts) != 3 {
			return
		}
		var data struct {
			B [][]string `json:"b"`
			A [][]string `json:"a"`
		}
		if err := json.Unmarshal(base.Data, &data); err == nil {
			ws.obCache.Update(parts[2], base.Type == "snapshot", data.B, data.A)
		}
		return
	}

	if strings.HasPrefix(base.Topic, "kline.") {
		var data []map[string]interface{}
		if err := json.Unmarshal(base.Data, &data); err == nil {
			if candle, symbol, interval, ok := ParseWSKline(data); ok {
				ws.klineCache.UpdateWS(symbol, interval, candle)
			}
		}
	}
}
