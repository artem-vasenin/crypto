package indicators

import (
	"sc/models"
	"testing"
)

func candles(values ...float64) []models.Candle {
	out := make([]models.Candle, len(values))
	for i, v := range values {
		out[i] = models.Candle{Close: v, High: v + 0.1, Low: v - 0.1, Volume: 100}
	}
	return out
}

func TestRSIAndATR(t *testing.T) {
	c := candles(1, 1.1, 1.2, 1.1, 1.15, 1.2, 1.18, 1.25, 1.3, 1.28, 1.35, 1.4, 1.38, 1.45, 1.5, 1.48, 1.55)
	if RSI(c, 14) <= 0 || RSI(c, 14) > 100 {
		t.Fatalf("unexpected RSI: %.2f", RSI(c, 14))
	}
	if ATR(c, 14) <= 0 {
		t.Fatal("ATR should be positive")
	}
}

func TestVolumeRatio(t *testing.T) {
	c := candles(1, 2, 3, 4, 5, 6)
	for n := 0; n < len(c)-1; n++ {
		c[n].Volume = 100
	}
	c[len(c)-1].Volume = 200
	r := VolumeRatio(c, 5)
	if r != 2 {
		t.Fatalf("volume ratio = %.2f, want 2", r)
	}
}
