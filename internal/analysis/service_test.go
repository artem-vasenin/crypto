package analysis

import (
	"testing"
	"time"

	"universal-bybit-screener/models"
)

func TestFundingAverage24h(t *testing.T) {
	base := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	points := []models.FundingPoint{
		{Time: base.Add(-30 * time.Hour), Rate: 0.03},
		{Time: base.Add(-16 * time.Hour), Rate: 0.01},
		{Time: base.Add(-8 * time.Hour), Rate: 0.03},
		{Time: base, Rate: 0.02},
	}
	got := fundingAverage24h(points)
	want := (0.01 + 0.03 + 0.02) / 3
	if got != want {
		t.Fatalf("funding average 24h = %.6f, want %.6f", got, want)
	}
}

func TestSpreadPct(t *testing.T) {
	ticker := models.Ticker{LastPrice: 1, Bid1Price: 0.999, Ask1Price: 1.001}
	if got := spreadPct(ticker); got < 0.199 || got > 0.201 {
		t.Fatalf("spread = %.3f%%, want 0.2%%", got)
	}
}
