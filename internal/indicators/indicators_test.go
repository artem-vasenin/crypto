package indicators

import (
	"math"
	"testing"

	"bybit-screener/internal/bybit"
)

func sampleKlines() []bybit.Kline {
	closes := []float64{1, 2, 3, 2, 4, 5, 4, 6, 7, 6, 8, 9, 10, 9, 11, 12, 13, 12, 14, 15, 16}
	out := make([]bybit.Kline, len(closes))
	for i, c := range closes {
		out[i] = bybit.Kline{StartTime: int64(i), Open: c, High: c + 0.5, Low: c - 0.5, Close: c, Volume: float64(i + 1)}
	}
	return out
}

func TestRSIIsInRange(t *testing.T) {
	rsi := RSI(sampleKlines(), 14)
	if rsi < 0 || rsi > 100 {
		t.Fatalf("RSI out of range: %f", rsi)
	}
}

func TestEMAIsPositive(t *testing.T) {
	ema := EMA(sampleKlines(), 20)
	if ema <= 0 {
		t.Fatalf("EMA must be positive: %f", ema)
	}
}

func TestATRIsPositive(t *testing.T) {
	atr := ATR(sampleKlines(), 14)
	if atr <= 0 {
		t.Fatalf("ATR must be positive: %f", atr)
	}
}

func TestPearsonPerfectCorrelation(t *testing.T) {
	got := Pearson([]float64{1, 2, 3}, []float64{2, 4, 6})
	if math.Abs(got-1) > 1e-9 {
		t.Fatalf("expected 1, got %f", got)
	}
}
