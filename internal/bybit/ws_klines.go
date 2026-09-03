// internal/bybit/ws_klines.go
package bybit

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"universal-bybit-screener/models"
)

type SymbolKlines struct {
	mu     sync.RWMutex
	klines map[string][]models.Candle
}

type KlineCache struct {
	mu    sync.RWMutex
	cache map[string]*SymbolKlines
}

func NewKlineCache() *KlineCache {
	return &KlineCache{
		cache: make(map[string]*SymbolKlines),
	}
}

func (kc *KlineCache) Warmup(symbol, interval string, candles []models.Candle) {
	kc.mu.Lock()
	defer kc.mu.Unlock()

	sk, exists := kc.cache[symbol]
	if !exists {
		sk = &SymbolKlines{klines: make(map[string][]models.Candle)}
		kc.cache[symbol] = sk
	}

	sk.mu.Lock()
	defer sk.mu.Unlock()
	sk.klines[interval] = candles
}

func (kc *KlineCache) UpdateWS(symbol, interval string, candle models.Candle) {
	kc.mu.Lock()
	sk, exists := kc.cache[symbol]
	if !exists {
		sk = &SymbolKlines{klines: make(map[string][]models.Candle)}
		kc.cache[symbol] = sk
	}
	kc.mu.Unlock()

	sk.mu.Lock()
	defer sk.mu.Unlock()

	arr := sk.klines[interval]
	if len(arr) == 0 {
		sk.klines[interval] = []models.Candle{candle}
		return
	}

	lastIdx := len(arr) - 1
	if arr[lastIdx].Time.Equal(candle.Time) {
		arr[lastIdx] = candle
	} else if candle.Time.After(arr[lastIdx].Time) {
		if len(arr) >= 300 {
			arr = append(arr[1:], candle)
		} else {
			arr = append(arr, candle)
		}
		sk.klines[interval] = arr
	}
}

func (kc *KlineCache) Get(symbol, interval string) []models.Candle {
	kc.mu.RLock()
	sk, exists := kc.cache[symbol]
	kc.mu.RUnlock()

	if !exists {
		return nil
	}

	sk.mu.RLock()
	defer sk.mu.RUnlock()

	arr := sk.klines[interval]
	res := make([]models.Candle, len(arr))
	copy(res, arr)
	return res
}

func ParseWSKline(data []map[string]interface{}) (models.Candle, string, string, bool) {
	if len(data) == 0 {
		return models.Candle{}, "", "", false
	}
	item := data[0]

	symbol, _ := item["symbol"].(string)
	interval, _ := item["interval"].(string)
	startMS, _ := strconv.ParseInt(fmtString(item["start"]), 10, 64)

	return models.Candle{
		Time:     time.UnixMilli(startMS).UTC(),
		Open:     parseFloat(item["open"]),
		High:     parseFloat(item["high"]),
		Low:      parseFloat(item["low"]),
		Close:    parseFloat(item["close"]),
		Volume:   parseFloat(item["volume"]),
		Turnover: parseFloat(item["turnover"]),
	}, symbol, interval, true
}

func parseFloat(val interface{}) float64 {
	switch v := val.(type) {
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case float64:
		return v
	default:
		return 0
	}
}

func fmtString(val interface{}) string {
	if s, ok := val.(string); ok {
		return s
	}
	if val != nil {
		return fmt.Sprintf("%v", val)
	}
	return ""
}
