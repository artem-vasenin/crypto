package bybit

import (
	"testing"
)

func TestPublicWSStreamReplaceSymbols(t *testing.T) {
	ws := NewPublicWSStream(NewOrderBookCache(), NewKlineCache(), 50)

	ws.ReplaceSymbols([]string{"BTCUSDT", "ETHUSDT"})
	if len(ws.subTopics) != 12 {
		t.Fatalf("expected 12 topics, got %d", len(ws.subTopics))
	}

	ws.ReplaceSymbols([]string{"ETHUSDT", "SOLUSDT"})
	if len(ws.subTopics) != 12 {
		t.Fatalf("expected 12 topics after replacement, got %d", len(ws.subTopics))
	}
	if ws.subTopics["orderbook.50.BTCUSDT"] {
		t.Fatal("BTCUSDT topics must be removed from the desired subscription set")
	}
	if !ws.subTopics["orderbook.50.ETHUSDT"] {
		t.Fatal("ETHUSDT order book topic must remain subscribed")
	}
	if !ws.subTopics["orderbook.50.SOLUSDT"] {
		t.Fatal("SOLUSDT order book topic must be subscribed")
	}
}

func TestOrderBookCacheFreshness(t *testing.T) {
	cache := NewOrderBookCache()
	cache.Update("BTCUSDT", true, [][]string{{"100", "1"}}, [][]string{{"101", "1"}})

	metrics := cache.GetMetrics("BTCUSDT")
	if metrics.Levels == 0 {
		t.Fatal("expected order book metrics after snapshot")
	}
	if cache.Age("BTCUSDT") < 0 {
		t.Fatal("order book age cannot be negative")
	}

}
