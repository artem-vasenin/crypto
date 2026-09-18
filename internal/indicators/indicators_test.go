package indicators

import (
	"math"
	"testing"
)

func TestEMA(t *testing.T) {
	x := EMA([]float64{1, 2, 3, 4, 5}, 3)
	if math.Abs(x[len(x)-1]-4.0625) > 1e-9 {
		t.Fatal(x)
	}
}
func TestEfficiencyRatio(t *testing.T) {
	if EfficiencyRatio([]float64{1, 2, 3, 4, 5}, 4) < .99 {
		t.Fatal("trend ER должен быть близок к 1")
	}
}
func TestRSI(t *testing.T) {
	if RSI([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, 14) < 99 {
		t.Fatal("RSI uptrend")
	}
}
