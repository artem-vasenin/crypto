package screening

import (
	"candidate-screener/internal/config"
	"candidate-screener/internal/domain"
	"fmt"
	"math"
)

// Futures оценивает пригодность для более глубокого directional-анализа через иерархию evidence.
func Futures(s domain.MarketSnapshot, dir domain.Direction, c config.Config) (domain.Candidate, bool) {
	x := base(s)
	if s.DataQuality != "GOOD" || s.Direction == domain.DirectionUnknown || s.Direction == domain.DirectionTransition || s.Direction == domain.DirectionSideways {
		return x, false
	}
	if s.MTFAlignment != "ALIGNED" && c.Thresholds.MTFConflictBlock {
		return x, false
	}
	if s.Direction != dir || s.Strength == domain.StrengthWeak {
		return x, false
	}
	one := s.TF["1h"]
	x.Decision = "CANDIDATE"
	x.PrimaryEvidence = append(x.PrimaryEvidence, fmt.Sprintf("1h structure=%s", one.Structure), fmt.Sprintf("1h center drift=%.2f ATR", one.CenterDriftATR), fmt.Sprintf("MTF=%s", s.MTFAlignment))
	if one.ADX >= c.Thresholds.ADXTrend {
		x.Confirmations = append(x.Confirmations, "ADX подтверждает направленность")
	}
	if one.VolumeRatio >= c.Thresholds.VolumeExpansionRatio {
		x.Confirmations = append(x.Confirmations, "Объём выше предыдущего окна")
	}
	if math.Abs(one.MomentumPct) > 5 {
		x.RiskFlags = append(x.RiskFlags, "Сильное краткосрочное растяжение цены; нужен контроль точки входа")
	}
	return x, true
}

// Grid сначала применяет hard gates устойчивости equilibrium и только затем допускает направление.
// Положительное направление не может компенсировать drifting/expanding range: это главный урок
// AVAX/WIF/PEPE/VVV и первого regression-кейса новой версии ADAUSDT.
func Grid(s domain.MarketSnapshot, dir domain.Direction, inst domain.Instrument, c config.Config) (domain.Candidate, bool) {
	x := base(s)
	if s.DataQuality != "GOOD" || s.Direction != dir || s.MTFAlignment != "ALIGNED" {
		return x, false
	}
	one := s.TF["1h"]
	if s.Strength == domain.StrengthStrong && s.Dynamics == domain.DynamicsAccelerating {
		return x, false
	}
	if math.Abs(one.CenterDriftATR) > c.Thresholds.DriftATRStrong {
		return x, false
	}
	if c.Thresholds.GridRollingMidDriftPctMax > 0 && math.Abs(one.RollingMidDriftPct) > c.Thresholds.GridRollingMidDriftPctMax {
		return x, false
	}
	if c.Thresholds.GridRollingMidDriftATRMax > 0 && math.Abs(one.RollingMidDriftATR) > c.Thresholds.GridRollingMidDriftATRMax {
		return x, false
	}
	if c.Thresholds.GridRangeExpansionPctMax > 0 && one.RangeWidthChangePct > c.Thresholds.GridRangeExpansionPctMax {
		return x, false
	}
	if c.Thresholds.GridEfficiencyMax > 0 && one.Efficiency > c.Thresholds.GridEfficiencyMax {
		return x, false
	}
	if c.Thresholds.GridMinMidpointCrossings > 0 && one.MidpointCrossings < c.Thresholds.GridMinMidpointCrossings {
		return x, false
	}
	if c.Thresholds.GridMinMeanReversionRatio > 0 && one.MeanReversionRatio < c.Thresholds.GridMinMeanReversionRatio {
		return x, false
	}
	if c.GridMaxPriceUSDT > 0 && s.Price > c.GridMaxPriceUSDT {
		return x, false
	}
	if !gridCapitalSuitable(s.Price, inst, c) {
		return x, false
	}
	x.Decision = "CANDIDATE"
	x.PrimaryEvidence = append(x.PrimaryEvidence,
		"Направление MTF согласовано",
		fmt.Sprintf("1h rolling midpoint drift %.2f%% / %.2f ATR допустим для Grid", one.RollingMidDriftPct, one.RollingMidDriftATR),
		fmt.Sprintf("1h range width change %.1f%% не показывает опасный expansion", one.RangeWidthChangePct),
		fmt.Sprintf("1h mean reversion %.2f, midpoint crossings %.0f", one.MeanReversionRatio, one.MidpointCrossings))
	if one.ADX < c.Thresholds.ADXStrong {
		x.Confirmations = append(x.Confirmations, "Трендовость не экстремальна для сетки")
	}
	return x, true
}

// gridCapitalSuitable грубо проверяет, возможно ли распределить небольшой капитал по минимальному числу уровней.
func gridCapitalSuitable(price float64, i domain.Instrument, c config.Config) bool {
	if price <= 0 {
		return false
	}
	minOrder := math.Max(i.MinNotional, price*i.MinOrderQty)
	if minOrder <= 0 {
		minOrder = 5
	}
	return minOrder*float64(c.GridMinLevels) <= c.GridReferenceCapitalUSDT
}
func base(s domain.MarketSnapshot) domain.Candidate {
	one := s.TF["1h"]
	return domain.Candidate{Symbol: s.Symbol, Direction: s.Direction, Strength: s.Strength, Dynamics: s.Dynamics, MTFAlignment: s.MTFAlignment, PrimaryEvidence: []string{}, Confirmations: []string{}, RiskFlags: append([]string{}, s.Conflicts...), Metrics: map[string]float64{
		"price": s.Price, "turnover_24h": s.Turnover24h, "funding_rate": s.FundingRate,
		"rolling_mid_drift_pct_1h": one.RollingMidDriftPct, "rolling_mid_drift_atr_1h": one.RollingMidDriftATR,
		"range_width_change_pct_1h": one.RangeWidthChangePct, "midpoint_crossings_1h": one.MidpointCrossings,
		"mean_reversion_ratio_1h": one.MeanReversionRatio,
	}}
}
