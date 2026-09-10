package bybit

import (
	"math"
	"sort"
	"strconv"
	"sync"

	"universal-bybit-screener/models"
)

type LocalOrderBook struct {
	bids map[float64]float64
	asks map[float64]float64
}

type OrderBookCache struct {
	mu    sync.RWMutex
	books map[string]*LocalOrderBook
}

func NewOrderBookCache() *OrderBookCache {
	return &OrderBookCache{
		books: make(map[string]*LocalOrderBook),
	}
}

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

// GetMetrics высчитывает Imbalance СТРОГО по Top-10 уровням спреда (отсекая спуфинг)
func (c *OrderBookCache) GetMetrics(symbol string) models.OrderBookMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	book, exists := c.books[symbol]
	if !exists || len(book.bids) == 0 || len(book.asks) == 0 {
		return models.OrderBookMetrics{}
	}

	type level struct {
		price float64
		size  float64
	}

	bids := make([]level, 0, len(book.bids))
	for p, s := range book.bids {
		bids = append(bids, level{price: p, size: s})
	}
	sort.Slice(bids, func(i, j int) bool { return bids[i].price > bids[j].price })

	asks := make([]level, 0, len(book.asks))
	for p, s := range book.asks {
		asks = append(asks, level{price: p, size: s})
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].price < asks[j].price })

	var bidNotional, askNotional float64
	depthBids := int(math.Min(10, float64(len(bids))))
	depthAsks := int(math.Min(10, float64(len(asks))))

	for i := 0; i < depthBids; i++ {
		bidNotional += bids[i].price * bids[i].size
	}
	for i := 0; i < depthAsks; i++ {
		askNotional += asks[i].price * asks[i].size
	}

	total := bidNotional + askNotional
	imbalance := 0.0
	if total > 0 {
		imbalance = (bidNotional - askNotional) / total * 100.0
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
		Levels:       depthBids + depthAsks,
	}
}
