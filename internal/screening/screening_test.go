package screening

import (
	"candidate-screener/internal/config"
	"candidate-screener/internal/domain"
	"testing"
)

func cfg() config.Config {
	return config.Config{MinHistoryBars: 100, GridReferenceCapitalUSDT: 50, GridMinLevels: 8, GridMaxPriceUSDT: 250, Thresholds: config.Thresholds{ADXTrend: 22, ADXStrong: 30, DriftATRWeak: .35, DriftATRStrong: .9, EfficiencyTrend: .32, VolumeExpansionRatio: 1.25, MTFConflictBlock: true}}
}
func TestFuturesRejectConflict(t *testing.T) {
	s := domain.MarketSnapshot{Direction: domain.DirectionUp, Strength: domain.StrengthStrong, MTFAlignment: "CONFLICT", DataQuality: "GOOD", TF: map[string]domain.TimeframeFeatures{"1h": {}}}
	if _, ok := Futures(s, domain.DirectionUp, cfg()); ok {
		t.Fatal("конфликт не должен проходить hard gate")
	}
}
func TestGridRejectStrongAcceleration(t *testing.T) {
	s := domain.MarketSnapshot{Price: 10, Direction: domain.DirectionUp, Strength: domain.StrengthStrong, Dynamics: domain.DynamicsAccelerating, MTFAlignment: "ALIGNED", DataQuality: "GOOD", TF: map[string]domain.TimeframeFeatures{"1h": {}}}
	if _, ok := Grid(s, domain.DirectionUp, domain.Instrument{MinNotional: 1}, cfg()); ok {
		t.Fatal("ускоряющийся сильный режим должен быть NO_GRID")
	}
}
