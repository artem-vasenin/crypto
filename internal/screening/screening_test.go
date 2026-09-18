package screening

import (
	"candidate-screener/internal/config"
	"candidate-screener/internal/domain"
	"testing"
)

func cfg() config.Config {
	return config.Config{MinHistoryBars: 100, GridReferenceCapitalUSDT: 50, GridMinLevels: 8, GridMaxPriceUSDT: 250, Thresholds: config.Thresholds{
		ADXTrend: 22, ADXStrong: 30, DriftATRWeak: .35, DriftATRStrong: .9, EfficiencyTrend: .32, VolumeExpansionRatio: 1.25, MTFConflictBlock: true,
		GridRollingMidDriftPctMax: 3, GridRollingMidDriftATRMax: 1.5, GridRangeExpansionPctMax: 35, GridEfficiencyMax: .55, GridMinMidpointCrossings: 2, GridMinMeanReversionRatio: .45,
	}}
}
func TestFuturesRejectConflict(t *testing.T) {
	s := domain.MarketSnapshot{Direction: domain.DirectionUp, Strength: domain.StrengthStrong, MTFAlignment: "CONFLICT", DataQuality: "GOOD", TF: map[string]domain.TimeframeFeatures{"1h": {}}}
	if _, ok := Futures(s, domain.DirectionUp, cfg()); ok {
		t.Fatal("конфликт не должен проходить hard gate")
	}
}
func TestGridRejectStrongAcceleration(t *testing.T) {
	s := domain.MarketSnapshot{Price: 10, Direction: domain.DirectionUp, Strength: domain.StrengthStrong, Dynamics: domain.DynamicsAccelerating, MTFAlignment: "ALIGNED", DataQuality: "GOOD", TF: map[string]domain.TimeframeFeatures{"1h": {MidpointCrossings: 5, MeanReversionRatio: 1}}}
	if _, ok := Grid(s, domain.DirectionUp, domain.Instrument{MinNotional: 1}, cfg()); ok {
		t.Fatal("ускоряющийся сильный режим должен быть NO_GRID")
	}
}
func TestGridRejectRollingMidpointDrift(t *testing.T) {
	s := goodGridSnapshot()
	f := s.TF["1h"]
	f.RollingMidDriftPct = 6.75
	s.TF["1h"] = f
	if _, ok := Grid(s, domain.DirectionUp, domain.Instrument{MinNotional: 1}, cfg()); ok {
		t.Fatal("сильный rolling midpoint drift должен быть hard block независимо от LONG direction")
	}
}
func TestGridRejectRangeExpansion(t *testing.T) {
	s := goodGridSnapshot()
	f := s.TF["1h"]
	f.RangeWidthChangePct = 52.9
	s.TF["1h"] = f
	if _, ok := Grid(s, domain.DirectionUp, domain.Instrument{MinNotional: 1}, cfg()); ok {
		t.Fatal("опасное расширение range должно блокировать Grid")
	}
}
func TestGridRejectPoorMeanReversion(t *testing.T) {
	s := goodGridSnapshot()
	f := s.TF["1h"]
	f.MidpointCrossings = 1
	f.MeanReversionRatio = .2
	s.TF["1h"] = f
	if _, ok := Grid(s, domain.DirectionUp, domain.Instrument{MinNotional: 1}, cfg()); ok {
		t.Fatal("локальные колебания без возвратов к midpoint не доказывают grid regime")
	}
}
func TestGridAcceptStationaryDirectionalRange(t *testing.T) {
	if _, ok := Grid(goodGridSnapshot(), domain.DirectionUp, domain.Instrument{MinNotional: 1}, cfg()); !ok {
		t.Fatal("здоровый stationary directional range не должен блокироваться")
	}
}
func goodGridSnapshot() domain.MarketSnapshot {
	return domain.MarketSnapshot{Price: 10, Direction: domain.DirectionUp, Strength: domain.StrengthModerate, Dynamics: domain.DynamicsStable, MTFAlignment: "ALIGNED", DataQuality: "GOOD", TF: map[string]domain.TimeframeFeatures{"1h": {CenterDriftATR: .4, RollingMidDriftPct: 1.2, RollingMidDriftATR: .7, RangeWidthChangePct: 10, Efficiency: .3, MidpointCrossings: 5, MeanReversionRatio: .75}}}
}
