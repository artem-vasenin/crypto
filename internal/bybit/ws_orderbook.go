// internal/bybit/ws_orderbook.go
package bybit

import (
	"strconv"
	"sync"

	"universal-bybit-screener/models"
)

const defaultWsURL = "wss://stream.bybit.com/v5/public/linear"

// LocalOrderBook хранит L2-книгу ордеров конкретного тикера в ОЗУ
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

// GetMetrics за O(1) высчитывает объем Bid/Ask ликвидности и имбаланс стакана
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
