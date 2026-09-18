package features

import (
	"candidate-screener/internal/domain"
	"math"
	"testing"
	"time"
)

// candlesRange создаёт deterministic свечи для проверки equilibrium без внешнего API.
func candlesRange(n int, centerStart, centerStep, amplitude float64) []domain.Candle {
	out := make([]domain.Candle, n)
	for i := range out {
		center := centerStart + float64(i)*centerStep
		close := center + amplitude*math.Sin(float64(i)*math.Pi/4)
		out[i] = domain.Candle{OpenTime: time.Unix(int64(i*3600), 0), Open: close, High: close + amplitude*.15, Low: close - amplitude*.15, Close: close, Volume: 100}
	}
	return out
}

func TestEquilibriumStationaryRange(t *testing.T) {
	f := ExtractTimeframe("1h", candlesRange(180, 100, 0, 2))
	if math.Abs(f.RollingMidDriftPct) > .5 {
		t.Fatalf("stationary range получил drift %.2f%%", f.RollingMidDriftPct)
	}
	if f.MidpointCrossings < 5 {
		t.Fatalf("ожидались повторные пересечения midpoint, получено %.0f", f.MidpointCrossings)
	}
	if f.MeanReversionRatio < .5 {
		t.Fatalf("ожидалась mean reversion, получено %.2f", f.MeanReversionRatio)
	}
}

func TestEquilibriumDetectsMigratingRange(t *testing.T) {
	f := ExtractTimeframe("1h", candlesRange(180, 100, .12, 1.5))
	if f.RollingMidDriftPct < 3 {
		t.Fatalf("drifting range не распознан: %.2f%%", f.RollingMidDriftPct)
	}
}
