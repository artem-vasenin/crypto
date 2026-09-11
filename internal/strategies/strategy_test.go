package strategies

import (
	"testing"

	"universal-bybit-screener/models"
)

func TestAllStrategiesAreRegistered(t *testing.T) {
	for _, name := range []string{"long", "short", "long-grid", "short-grid", "neutral-grid"} {
		if _, err := New(name); err != nil {
			t.Fatalf("strategy %q is not registered: %v", name, err)
		}
	}
}

func TestNeutralGridRejectsDirectionalTrend(t *testing.T) {
	candidate := models.Candidate{}
	candidate.Structure = map[string]models.Structure{
		"1h": {HighState: "HH", LowState: "HL"},
		"4h": {HighState: "HH", LowState: "HL"},
	}

	result := NeutralGrid{}.Evaluate(&candidate)
	if result.Status != "reject" {
		t.Fatalf("expected neutral grid rejection, got %+v", result)
	}
}
