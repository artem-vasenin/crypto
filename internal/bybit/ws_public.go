// internal/bybit/ws_public.go
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

type PublicWSStream struct {
	obCache    *OrderBookCache
	klineCache *KlineCache
	conn       *websocket.Conn
	mu         sync.Mutex
	subTopics  map[string]bool
	isCtxDone  bool
}

func NewPublicWSStream(ob *OrderBookCache, kc *KlineCache) *PublicWSStream {
	return &PublicWSStream{
		obCache:    ob,
		klineCache: kc,
		subTopics:  make(map[string]bool),
	}
}

func (ws *PublicWSStream) Start(ctx context.Context, symbols []string) error {
	ws.mu.Lock()
	for _, s := range symbols {
		ws.subTopics[fmt.Sprintf("orderbook.50.%s", s)] = true
		ws.subTopics[fmt.Sprintf("kline.15.%s", s)] = true
		ws.subTopics[fmt.Sprintf("kline.60.%s", s)] = true
		ws.subTopics[fmt.Sprintf("kline.240.%s", s)] = true
	}
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
			log.Printf("[WS WARN] Public Stream dial error: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		ws.mu.Lock()
		ws.conn = conn
		ws.mu.Unlock()

		ws.subscribeAll()

		pingCtx, pingCancel := context.WithCancel(ctx)
		go ws.keepAlive(pingCtx, conn)

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				pingCancel()
				ws.mu.Lock()
				if !ws.isCtxDone {
					log.Printf("[WS WARN] Public Stream disconnect: %v. Reconnecting...", err)
				}
				ws.mu.Unlock()
				_ = conn.Close()
				break
			}

			ws.parseMessage(msgBytes)
		}

		pingCancel()
		time.Sleep(2 * time.Second)
	}
}

func (ws *PublicWSStream) subscribeAll() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.conn == nil || len(ws.subTopics) == 0 {
		return
	}

	topics := make([]string, 0, len(ws.subTopics))
	for t := range ws.subTopics {
		topics = append(topics, t)
	}

	batchSize := 100
	for i := 0; i < len(topics); i += batchSize {
		end := i + batchSize
		if end > len(topics) {
			end = len(topics)
		}

		subMsg := map[string]interface{}{
			"op":   "subscribe",
			"args": topics[i:end],
		}
		_ = ws.conn.WriteJSON(subMsg)
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
			if conn != nil {
				_ = ws.conn.WriteJSON(map[string]string{"op": "ping"})
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
		if len(parts) == 3 {
			var obData struct {
				B [][]string `json:"b"`
				A [][]string `json:"a"`
			}
			if err := json.Unmarshal(base.Data, &obData); err == nil {
				ws.obCache.Update(parts[2], base.Type == "snapshot", obData.B, obData.A)
			}
		}
	} else if strings.HasPrefix(base.Topic, "kline.") {
		var klineData []map[string]interface{}
		if err := json.Unmarshal(base.Data, &klineData); err == nil {
			if candle, symbol, interval, ok := ParseWSKline(klineData); ok {
				ws.klineCache.UpdateWS(symbol, interval, candle)
			}
		}
	}
}
