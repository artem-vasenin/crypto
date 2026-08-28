package structure

import (
	"sc/models"
	"testing"
)

func TestLevelsAreSortedNearestFirst(t *testing.T) {
	s := models.Structure{
		Highs: []models.Pivot{{Price: 1.30}, {Price: 1.10}, {Price: 1.50}},
		Lows:  []models.Pivot{{Price: .70}, {Price: .90}, {Price: .50}},
	}
	l := Levels(s, 1)
	if l.NearestResistance != 1.10 {
		t.Fatalf("nearest resistance = %.2f, want 1.10", l.NearestResistance)
	}
	if l.NearestSupport != .90 {
		t.Fatalf("nearest support = %.2f, want 0.90", l.NearestSupport)
	}
	if l.Resistance[0] != 1.10 || l.Support[0] != .90 {
		t.Fatal("levels are not sorted nearest first")
	}
	if l.RangePositionPct <= 0 || l.RangePositionPct >= 100 {
		t.Fatal("range position was not calculated")
	}
}

func TestApplyATR(t *testing.T) {
	l := Levels(models.Structure{Highs: []models.Pivot{{Price: 1.2}}, Lows: []models.Pivot{{Price: .8}}}, 1)
	l = ApplyATR(l, .1, 1)
	if l.RangeToATR1h < 3.999 || l.RangeToATR1h > 4.001 {
		t.Fatalf("range/ATR = %.2f, want 4", l.RangeToATR1h)
	}
}
