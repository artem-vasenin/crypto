package strategies

import (
	"sc/models"
	"testing"
)

func baseData() (models.MarketData, models.Indicators, map[string]models.Structure, models.Levels) {
	m := models.MarketData{Ticker: models.Ticker{LastPrice: 1, Price24hPcnt: 1, Bid1Price: .999, Ask1Price: 1.001, FundingRate: 0.0001}}
	i := models.Indicators{RSI1h: 50, RSI4h: 50, ATR1h: .03, ATR1hPct: 3, VolumeRatio1h: 1}
	s := map[string]models.Structure{
		"1h": {HighState: "EQ", LowState: "EQ"},
		"4h": {HighState: "EQ", LowState: "EQ"},
	}
	l := models.Levels{NearestSupport: .85, NearestResistance: 1.15, RangeWidthPct: 35.29, RangePositionPct: 50, RangeToATR1h: 10}
	return m, i, s, l
}

func TestNamesAndNew(t *testing.T) {
	want := []string{"short-grid", "short", "long-grid", "long", "neutral-grid"}
	got := Names()
	if len(got) != len(want) {
		t.Fatalf("Names length = %d, want %d", len(got), len(want))
	}
	for n, name := range want {
		if got[n] != name {
			t.Errorf("Names[%d] = %q, want %q", n, got[n], name)
		}
		st, err := New(name)
		if err != nil {
			t.Fatalf("New(%q): %v", name, err)
		}
		if st.Name() != name {
			t.Errorf("Name() = %q, want %q", st.Name(), name)
		}
	}
	if _, err := New("unknown"); err == nil {
		t.Fatal("New(unknown) expected error")
	}
}

func TestNeutralGridFavorsRange(t *testing.T) {
	m, i, s, l := baseData()
	result := NeutralGrid{}.Evaluate(m, i, s, l)
	if result.Score < 70 {
		t.Fatalf("neutral grid score = %.2f, expected >= 70", result.Score)
	}
}

func TestNeutralGridPenalizesTrend(t *testing.T) {
	m, i, s, l := baseData()
	flat := NeutralGrid{}.Evaluate(m, i, s, l)
	s["1h"] = models.Structure{HighState: "HH", LowState: "HL"}
	s["4h"] = models.Structure{HighState: "HH", LowState: "HL"}
	trend := NeutralGrid{}.Evaluate(m, i, s, l)
	if trend.Score >= flat.Score {
		t.Fatalf("trending score %.2f should be below flat score %.2f", trend.Score, flat.Score)
	}
}

func TestDirectionalStrategiesDoNotPanicWithEmptyLevels(t *testing.T) {
	m := models.MarketData{Ticker: models.Ticker{LastPrice: 1, Price24hPcnt: 10}}
	i := models.Indicators{RSI1h: 55, RSI4h: 55, ATR1hPct: 2}
	s := map[string]models.Structure{"1h": {HighState: "HH", LowState: "HL"}, "4h": {HighState: "HH", LowState: "HL"}}
	for _, name := range Names() {
		st, _ := New(name)
		result := st.Evaluate(m, i, s, models.Levels{})
		if result.Score < 0 || result.Score > 100 {
			t.Errorf("%s score out of range: %.2f", name, result.Score)
		}
	}
}
