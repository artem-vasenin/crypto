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

func TestLongRejectsConflictingHigherTimeframes(t *testing.T) {
	candidate := models.Candidate{}
	candidate.Market.Change24h = 4
	candidate.Market.Change3d = 3
	candidate.Market.Change7d = 2
	candidate.Market.Turnover24h = 50_000_000
	candidate.Market.SpreadPct = 0.02
	candidate.Indicators.RSI5m = 60
	candidate.Indicators.RSI15m = 62
	candidate.Indicators.RSI1h = 52
	candidate.Indicators.RSI4h = 45
	candidate.Indicators.VolumeRatio1h = 1
	candidate.Levels.RangePositionPct = 25
	candidate.OrderBook.ImbalancePct = 10
	candidate.Derivatives.OpenInterestChange = 1
	candidate.Derivatives.FundingRate = 0
	candidate.Structure = map[string]models.Structure{
		"5m":  {HighState: "HH", LowState: "HL"},
		"15m": {HighState: "HH", LowState: "HL"},
		"30m": {HighState: "HH", LowState: "HL"},
		"1h":  {HighState: "LH", LowState: "LL"},
		"4h":  {HighState: "LH", LowState: "LL"},
	}

	result := Long{}.Evaluate(&candidate)
	if result.Decision.Eligible {
		t.Fatalf("expected long rejection, got %+v", result)
	}
	if result.Scores.EntryQuality.Score < 60 {
		t.Fatalf("test fixture should have strong entry quality, got %+v", result.Scores.EntryQuality)
	}
	if result.Scores.TrendQuality.Score >= 60 {
		t.Fatalf("expected weak trend quality, got %+v", result.Scores.TrendQuality)
	}
}

func TestShortCanBeEligibleWhenBlocksAlign(t *testing.T) {
	candidate := models.Candidate{}
	candidate.Market.Change24h = -4
	candidate.Market.Change3d = -4
	candidate.Market.Change7d = -5
	candidate.Market.Turnover24h = 50_000_000
	candidate.Market.SpreadPct = 0.02
	candidate.Indicators.RSI5m = 40
	candidate.Indicators.RSI15m = 42
	candidate.Indicators.RSI1h = 45
	candidate.Indicators.RSI4h = 42
	candidate.Indicators.VolumeRatio1h = 1
	candidate.Levels.RangePositionPct = 75
	candidate.OrderBook.ImbalancePct = -10
	candidate.Derivatives.OpenInterestChange = 1
	candidate.Derivatives.FundingRate = 0.0002
	candidate.Structure = map[string]models.Structure{
		"5m":  {HighState: "LH", LowState: "LL"},
		"15m": {HighState: "LH", LowState: "LL"},
		"30m": {HighState: "LH", LowState: "LL"},
		"1h":  {HighState: "LH", LowState: "LL"},
		"4h":  {HighState: "LH", LowState: "LL"},
	}

	result := Short{}.Evaluate(&candidate)
	if !result.Decision.Eligible {
		t.Fatalf("expected short eligibility, got %+v", result)
	}
}
