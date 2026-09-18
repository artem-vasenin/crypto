package features

import (
	"candidate-screener/internal/config"
	"candidate-screener/internal/domain"
	"candidate-screener/internal/indicator"
	"candidate-screener/internal/structure"
	"math"
)

// ExtractTimeframe превращает свечи в объективные признаки без торгового решения.
func ExtractTimeframe(tf string, c []domain.Candle) domain.TimeframeFeatures {
	f := domain.TimeframeFeatures{Timeframe: tf, Bars: len(c), Structure: structure.Classify(c, 2)}
	if len(c) < 60 {
		return f
	}
	h, l, cl, v := make([]float64, len(c)), make([]float64, len(c)), make([]float64, len(c)), make([]float64, len(c))
	for i, x := range c {
		h[i], l[i], cl[i], v[i] = x.High, x.Low, x.Close, x.Volume
	}
	f.LastPrice = cl[len(cl)-1]
	f.ATR = indicator.ATR(h, l, cl, 14)
	if f.LastPrice > 0 {
		f.ATRPct = 100 * f.ATR / f.LastPrice
	}
	f.PlusDI, f.MinusDI, f.ADX = indicator.DMIADX(h, l, cl, 14)
	f.Efficiency = indicator.EfficiencyRatio(cl, 24)
	f.Center = indicator.SMA(cl, 48)
	old := 0.0
	if len(cl) >= 96 {
		old = indicator.SMA(cl[len(cl)-48:], 48)
		old2 := indicator.SMA(cl[:len(cl)-48], min(48, len(cl)-48))
		if old2 > 0 {
			f.CenterDriftPct = 100 * (old - old2) / old2
		}
		if f.ATR > 0 {
			f.CenterDriftATR = (old - old2) / f.ATR
		}
	}
	base := cl[len(cl)-13]
	if base > 0 {
		f.MomentumPct = 100 * (f.LastPrice - base) / base
	}
	recentV := indicator.SMA(v, 12)
	prior := indicator.SMA(v[:len(v)-12], 12)
	if prior > 0 {
		f.VolumeRatio = recentV / prior
	}
	return f
}

// ClassifyMarket формирует direction/strength/dynamics и явно сохраняет MTF conflict.
func ClassifyMarket(s *domain.MarketSnapshot, c config.Config) {
	a, b, d := s.TF["15m"], s.TF["1h"], s.TF["4h"]
	if a.Bars < c.MinHistoryBars || b.Bars < c.MinHistoryBars || d.Bars < 100 {
		s.Direction = domain.DirectionUnknown
		s.DataQuality = "INSUFFICIENT"
		return
	}
	s.DataQuality = "GOOD"
	bull := func(x domain.TimeframeFeatures) bool {
		return x.Structure == domain.StructureBull && x.CenterDriftATR > c.Thresholds.DriftATRWeak && x.PlusDI > x.MinusDI
	}
	bear := func(x domain.TimeframeFeatures) bool {
		return x.Structure == domain.StructureBear && x.CenterDriftATR < -c.Thresholds.DriftATRWeak && x.MinusDI > x.PlusDI
	}
	bu := []bool{bull(a), bull(b), bull(d)}
	be := []bool{bear(a), bear(b), bear(d)}
	bc, sc := count(bu), count(be)
	if bc >= 2 && sc == 0 {
		s.Direction = domain.DirectionUp
		s.MTFAlignment = "ALIGNED"
	} else if sc >= 2 && bc == 0 {
		s.Direction = domain.DirectionDown
		s.MTFAlignment = "ALIGNED"
	} else if (bull(a) && bull(b) && bear(d)) || (bear(a) && bear(b) && bull(d)) {
		s.Direction = domain.DirectionTransition
		s.MTFAlignment = "TRANSITION"
		s.Conflicts = append(s.Conflicts, "Младшие TF устойчиво противоречат 4h: возможен regime transition")
	} else if bc > 0 && sc > 0 {
		s.Direction = domain.DirectionUnknown
		s.MTFAlignment = "CONFLICT"
		s.Conflicts = append(s.Conflicts, "Направленные признаки таймфреймов конфликтуют")
	} else {
		s.Direction = domain.DirectionSideways
		s.MTFAlignment = "MIXED"
	}
	main := b
	absdr := math.Abs(main.CenterDriftATR)
	if main.ADX >= c.Thresholds.ADXStrong && absdr >= c.Thresholds.DriftATRStrong && main.Efficiency >= c.Thresholds.EfficiencyTrend {
		s.Strength = domain.StrengthStrong
	} else if main.ADX >= c.Thresholds.ADXTrend && absdr >= c.Thresholds.DriftATRWeak {
		s.Strength = domain.StrengthModerate
	} else {
		s.Strength = domain.StrengthWeak
	}
	short := a
	long := b
	if math.Abs(short.CenterDriftATR) > math.Abs(long.CenterDriftATR)*1.25 || short.ADX > long.ADX+5 {
		s.Dynamics = domain.DynamicsAccelerating
	} else if math.Abs(short.CenterDriftATR)*1.25 < math.Abs(long.CenterDriftATR) && short.ADX+5 < long.ADX {
		s.Dynamics = domain.DynamicsDecelerating
	} else {
		s.Dynamics = domain.DynamicsStable
	}
	s.Evidence = append(s.Evidence, "Направление определяется структурой, drift центра и DMI на нескольких TF; единый score не используется")
}
func count(v []bool) int {
	n := 0
	for _, x := range v {
		if x {
			n++
		}
	}
	return n
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
