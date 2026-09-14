package execution

import (
	"testing"
	"time"
)

func TestPublicTickerStoresFreshMarkPrice(t *testing.T) {
	ws := NewWSEngine("", "", false, nil, nil, nil, nil)
	ws.parsePublicMessage([]byte(`{"topic":"tickers.BTCUSDT","data":{"symbol":"BTCUSDT","lastPrice":"100.5","markPrice":"100.4"}}`))

	price, age, fresh := ws.GetFreshMarkPrice("BTCUSDT", time.Second)
	if !fresh {
		t.Fatalf("expected fresh mark price, got age=%s", age)
	}
	if price != 100.4 {
		t.Fatalf("expected mark price 100.4, got %.4f", price)
	}
}

func TestPublicTickerWithoutMarkPriceIsIgnoredForTrailing(t *testing.T) {
	ws := NewWSEngine("", "", false, nil, nil, nil, nil)
	ws.parsePublicMessage([]byte(`{"topic":"tickers.BTCUSDT","data":{"symbol":"BTCUSDT","lastPrice":"100.5"}}`))

	if _, _, fresh := ws.GetFreshMarkPrice("BTCUSDT", time.Second); fresh {
		t.Fatal("ticker without mark price must not be considered safe for mark-price trailing")
	}
}

func TestPublicTickerDeltaKeepsUnchangedMarkPrice(t *testing.T) {
	ws := NewWSEngine("", "", false, nil, nil, nil, nil)
	ws.parsePublicMessage([]byte(`{"topic":"tickers.BTCUSDT","type":"snapshot","data":{"symbol":"BTCUSDT","lastPrice":"100.5","markPrice":"100.4"}}`))
	ws.parsePublicMessage([]byte(`{"topic":"tickers.BTCUSDT","type":"delta","data":{"symbol":"BTCUSDT","lastPrice":"100.6"}}`))

	price, _, fresh := ws.GetFreshMarkPrice("BTCUSDT", time.Second)
	if !fresh || price != 100.4 {
		t.Fatalf("delta must retain unchanged mark price, got price=%.4f fresh=%v", price, fresh)
	}
}
