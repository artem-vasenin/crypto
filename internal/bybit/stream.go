// internal/bybit/stream.go
package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const wsURL = "wss://stream.bybit.com/v5/public/linear"

type StreamManager struct {
	state *MarketState
	conn  *websocket.Conn
	mu    sync.Mutex
	subs  []string
}

type wsMessage struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Data  struct {
		B [][]string `json:"b"`
		A [][]string `json:"a"`
	} `json:"data"`
}

func NewStreamManager(state *MarketState) *StreamManager {
	return &StreamManager{
		state: state,
	}
}

func (sm *StreamManager) Start(ctx context.Context, topics []string) error {
	sm.subs = topics
	if err := sm.connect(); err != nil {
		return err
	}

	go sm.readPump(ctx)
	go sm.pingPump(ctx)

	return nil
}

func (sm *StreamManager) connect() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	sm.conn = conn

	// Subscribe to topics
	if len(sm.subs) > 0 {
		req := map[string]interface{}{
			"op":   "subscribe",
			"args": sm.subs,
		}
		if err := sm.conn.WriteJSON(req); err != nil {
			return fmt.Errorf("ws subscribe: %w", err)
		}
	}
	return nil
}

func (sm *StreamManager) readPump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			sm.close()
			return
		default:
			sm.mu.Lock()
			conn := sm.conn
			sm.mu.Unlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WS read error: %v, reconnecting...", err)
				sm.reconnect()
				continue
			}

			var wsMsg wsMessage
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				continue
			}

			if strings.HasPrefix(wsMsg.Topic, "orderbook.") {
				parts := strings.Split(wsMsg.Topic, ".")
				if len(parts) >= 3 {
					symbol := parts[2]
					isSnapshot := wsMsg.Type == "snapshot"
					sm.state.ApplyOrderBook(symbol, isSnapshot, wsMsg.Data.B, wsMsg.Data.A)
				}
			}
		}
	}
}

func (sm *StreamManager) pingPump(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second) // Bybit needs ping every 20s
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.mu.Lock()
			if sm.conn != nil {
				_ = sm.conn.WriteJSON(map[string]string{"op": "ping"})
			}
			sm.mu.Unlock()
		}
	}
}

func (sm *StreamManager) reconnect() {
	sm.close()
	time.Sleep(2 * time.Second) // Linear backoff for simplicity
	_ = sm.connect()
}

func (sm *StreamManager) close() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.conn != nil {
		_ = sm.conn.Close()
		sm.conn = nil
	}
}
