package analysis

import (
	"testing"
	"time"

	"universal-bybit-screener/internal/bybit"
)

func TestFreshOrderBookSymbols(t *testing.T) {
	cache := bybit.NewOrderBookCache()
	cache.Update("BTCUSDT", true, [][]string{{"100", "1"}}, [][]string{{"101", "1"}})
	cache.Update("ETHUSDT", true, [][]string{{"100", "1"}}, [][]string{{"101", "1"}})

	ready := freshOrderBookSymbols([]string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}, cache, 30*time.Second)
	if len(ready) != 2 {
		t.Fatalf("expected 2 fresh symbols, got %d", len(ready))
	}
	if ready[0] != "BTCUSDT" || ready[1] != "ETHUSDT" {
		t.Fatalf("unexpected ready symbols: %v", ready)
	}
}
