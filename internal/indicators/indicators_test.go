package indicators

import (
	"testing"
	"time"

	"universal-bybit-screener/models"
)

func TestEMAUsesClosedCandleCloses(t *testing.T) {
	candles := make([]models.Candle, 10)
	for i := range candles {
		candles[i] = models.Candle{Time: time.Unix(int64(i), 0), Close: float64(i + 1)}
	}
	got := EMA(candles, 3)
	if got <= 8 || got >= 10 {
		t.Fatalf("unexpected EMA value %.4f", got)
	}
}
