// internal/bybit/ws_orderbook.go
package bybit

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"universal-bybit-screener/models"

	"github.com/gorilla/websocket"
)

// const defaultWsURLTest = "wss://stream-testnet.bybit.com/v5/public/linear"
const defaultWsURL = "wss://stream.bybit.com/v5/public/linear"

// LocalOrderBook хранит L2-книгу ордеров конкретной монеты в памяти
type LocalOrderBook struct {
	bids map[float64]float64
	asks map[float64]float64
}

// OrderBookCache обеспечивает thread-safe доступ к стаканам всех отслеживаемых тикеров
type OrderBookCache struct {
	mu    sync.RWMutex
	books map[string]*LocalOrderBook
}

func NewOrderBookCache() *OrderBookCache {
	return &OrderBookCache{
		books: make(map[string]*LocalOrderBook),
	}
}

// Update применяет первичные Snapshot и инкрементальные Delta сообщения
func (c *OrderBookCache) Update(symbol string, isSnapshot bool, rawBids, rawAsks [][]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	book, exists := c.books[symbol]
	if !exists || isSnapshot {
		book = &LocalOrderBook{
			bids: make(map[float64]float64),
			asks: make(map[float64]float64),
		}
		c.books[symbol] = book
	}

	applySide := func(target map[float64]float64, items [][]string) {
		for _, item := range items {
			if len(item) < 2 {
				continue
			}
			price, _ := strconv.ParseFloat(item[0], 64)
			size, _ := strconv.ParseFloat(item[1], 64)

			// По правилам Bybit size == "0" означает удаление ценового уровня
			if size == 0 {
				delete(target, price)
			} else {
				target[price] = size
			}
		}
	}

	applySide(book.bids, rawBids)
	applySide(book.asks, rawAsks)
}

// GetMetrics за $O(1)$ высчитывает объем Bid/Ask ликвидности и имбаланс стакана
func (c *OrderBookCache) GetMetrics(symbol string) models.OrderBookMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	book, exists := c.books[symbol]
	if !exists {
		return models.OrderBookMetrics{}
	}

	var bidNotional, askNotional float64
	for p, s := range book.bids {
		bidNotional += p * s
	}
	for p, s := range book.asks {
		askNotional += p * s
	}

	total := bidNotional + askNotional
	imbalance := 0.0
	if total > 0 {
		imbalance = (bidNotional - askNotional) / total * 100
	}

	ratio := 0.0
	if askNotional > 0 {
		ratio = bidNotional / askNotional
	}

	return models.OrderBookMetrics{
		BidNotional:  bidNotional,
		AskNotional:  askNotional,
		ImbalancePct: imbalance,
		BidAskRatio:  ratio,
		Levels:       len(book.bids) + len(book.asks),
	}
}

// WSClient управляет WebSocket-соединением, подписками и пингами
type WSClient struct {
	cache *OrderBookCache
	conn  *websocket.Conn
	mu    sync.Mutex
}

func NewWSClient(cache *OrderBookCache) *WSClient {
	return &WSClient{cache: cache}
}

// SubscribeOrderBooks подключается к публичному каналу Bybit V5 и оформляет подписки
func (ws *WSClient) SubscribeOrderBooks(ctx context.Context, symbols []string) error {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, defaultWsURL, http.Header{})
	if err != nil {
		return fmt.Errorf("ws connection dial error: %w", err)
	}

	ws.conn = conn

	var args []string
	for _, s := range symbols {
		args = append(args, fmt.Sprintf("orderbook.50.%s", s))
	}

	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	if err := conn.WriteJSON(subMsg); err != nil {
		conn.Close()
		return fmt.Errorf("ws subscribe frame write error: %w", err)
	}

	go ws.readLoop(ctx)
	go ws.keepAlive(ctx)

	return nil
}

func (ws *WSClient) readLoop(ctx context.Context) {
	defer func() {
		ws.mu.Lock()
		if ws.conn != nil {
			_ = ws.conn.Close()
		}
		ws.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg struct {
				Topic string `json:"topic"`
				Type  string `json:"type"`
				Data  struct {
					B [][]string `json:"b"`
					A [][]string `json:"a"`
				} `json:"data"`
			}

			if err := ws.conn.ReadJSON(&msg); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[WARN] WS Read failure: %v. Reconnecting...", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if strings.HasPrefix(msg.Topic, "orderbook.") {
				parts := strings.Split(msg.Topic, ".")
				if len(parts) == 3 {
					symbol := parts[2]
					ws.cache.Update(symbol, msg.Type == "snapshot", msg.Data.B, msg.Data.A)
				}
			}
		}
	}
}

// keepAlive отправляет каждые 20 секунд ping-фрейм для поддержания соединения (Bybit Ping Threshold = 30s)
func (ws *WSClient) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ws.mu.Lock()
			if ws.conn != nil {
				_ = ws.conn.WriteJSON(map[string]string{"op": "ping"})
			}
			ws.mu.Unlock()
		}
	}
}
