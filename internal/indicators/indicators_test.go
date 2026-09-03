package indicators

import (
	"math"
	"testing"
)

func TestEMA(t *testing.T) {
	got := EMA([]float64{1, 2, 3, 4, 5}, 3)
	if math.Abs(got[len(got)-1]-4.0625) > 1e-9 {
		t.Fatalf("EMA=%v", got)
	}
}
func TestRSIUptrend(t *testing.T) {
	got := RSI([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, 14)
	if got < 99 {
		t.Fatalf("RSI=%v", got)
	}
}
func TestCorrelation(t *testing.T) {
	got := Correlation([]float64{1, 2, 3, 4}, []float64{2, 4, 6, 8})
	if math.Abs(got-1) > 1e-9 {
		t.Fatalf("corr=%v", got)
	}
}
