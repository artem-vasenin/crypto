package indicator

import (
	"math"
	"testing"
)

func TestEfficiencyRatioTrend(t *testing.T) {
	v := []float64{1, 2, 3, 4, 5, 6}
	if x := EfficiencyRatio(v, 5); math.Abs(x-1) > 1e-9 {
		t.Fatalf("ожидался ER=1, получено %v", x)
	}
}
func TestSMA(t *testing.T) {
	if x := SMA([]float64{1, 2, 3, 4}, 2); x != 3.5 {
		t.Fatalf("SMA=%v", x)
	}
}
