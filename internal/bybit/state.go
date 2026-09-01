// internal/bybit/state.go
package bybit

import (
	"sync"
	"universal-bybit-screener/models"
)

// L2Book хранит локальный слепок стакана.
type L2Book struct {
	Bids map[float64]float64
	Asks map[float64]float64
}

// MarketState управляет потокобезопасным in-memory кэшем.
type MarketState struct {
	mu    sync.RWMutex
	books map[string]*L2Book
}

func NewMarketState() *MarketState {
	return &MarketState{
		books: make(map[string]*L2Book),
	}
}

// ApplyOrderBook обрабатывает Snapshot и Delta сообщения от WebSocket.
func (s *MarketState) ApplyOrderBook(symbol string, isSnapshot bool, bids, asks [][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	book, exists := s.books[symbol]
	if !exists || isSnapshot {
		book = &L2Book{
			Bids: make(map[float64]float64),
			Asks: make(map[float64]float64),
		}
		s.books[symbol] = book
	}

	updateLevels := func(levels map[float64]float64, updates [][]string) {
		for _, level := range updates {
			if len(level) < 2 {
				continue
			}
			price := f(level[0])
			size := f(level[1])
			if size == 0 {
				delete(levels, price)
			} else {
				levels[price] = size
			}
		}
	}

	updateLevels(book.Bids, bids)
	updateLevels(book.Asks, asks)
}

// GetOrderBookMetrics рассчитывает Notional Value и Imbalance на лету.
func (s *MarketState) GetOrderBookMetrics(symbol string) models.OrderBookMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	book, exists := s.books[symbol]
	if !exists {
		return models.OrderBookMetrics{}
	}

	var bidNotional, askNotional float64
	for price, size := range book.Bids {
		bidNotional += price * size
	}
	for price, size := range book.Asks {
		askNotional += price * size
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
		Levels:       len(book.Bids) + len(book.Asks),
	}
}
