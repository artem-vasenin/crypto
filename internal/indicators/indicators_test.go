// internal/indicators/indicators_test.go
package indicators

import (
	"math"
	"testing"
	"time"
	"universal-bybit-screener/models"
)

func TestRSI(t *testing.T) {
	candles := make([]models.Candle, 20)
	now := time.Now()

	// Имитация постоянного роста цены
	for i := range candles {
		candles[i] = models.Candle{
			Time:  now.Add(time.Duration(i) * time.Minute),
			Close: float64(100 + i),
		}
	}

	rsi := RSI(candles, 14)
	if rsi != 100.0 {
		t.Errorf("Expected RSI 100.0 for pure upward trend, got %.2f", rsi)
	}

	// Имитация недостаточной длины выборки
	shortCandles := candles[:10]
	if got := RSI(shortCandles, 14); got != 0.0 {
		t.Errorf("Expected RSI 0.0 for insufficient candle array, got %.2f", got)
	}
}

func TestATR(t *testing.T) {
	candles := []models.Candle{
		{High: 10, Low: 5, Close: 8},
		{High: 12, Low: 6, Close: 11},
		{High: 15, Low: 9, Close: 14},
		{High: 14, Low: 8, Close: 10},
	}

	atr := ATR(candles, 2)
	if atr <= 0 || math.IsNaN(atr) {
		t.Errorf("Invalid ATR calculation result: %f", atr)
	}
}

func TestVolumeRatio(t *testing.T) {
	candles := []models.Candle{
		{Volume: 100},
		{Volume: 100},
		{Volume: 100},
		{Volume: 300}, // Спайк объема
	}

	ratio := VolumeRatio(candles, 3)
	expected := 3.0
	if math.Abs(ratio-expected) > 0.0001 {
		t.Errorf("Expected VolumeRatio %.2f, got %.2f", expected, ratio)
	}
}
