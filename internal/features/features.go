package features

import (
	"candidate-screener/internal/config"
	"candidate-screener/internal/domain"
	"candidate-screener/internal/indicator"
	"candidate-screener/internal/structure"
	"math"
)

// ExtractTimeframe превращает свечи в объективные признаки без торгового решения.
// Помимо локального trend context здесь рассчитывается rolling equilibrium: он нужен Grid,
// чтобы не путать частые колебания с диапазоном, который целиком переезжает вверх/вниз.
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
	if len(cl) >= 96 {
		newCenter := indicator.SMA(cl[len(cl)-48:], 48)
		oldCenter := indicator.SMA(cl[len(cl)-96:len(cl)-48], 48)
		if oldCenter > 0 {
			f.CenterDriftPct = 100 * (newCenter - oldCenter) / oldCenter
		}
		if f.ATR > 0 {
			f.CenterDriftATR = (newCenter - oldCenter) / f.ATR
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
	applyEquilibriumFeatures(&f, h, l, cl)
	return f
}

// applyEquilibriumFeatures оценивает миграцию midpoint, изменение ширины range и возвраты к центру.
// Окно 48 баров намеренно одинаково в барах: смысл признака нормализуется timeframe и ATR,
// а thresholds остаются калибруемыми и не являются универсальными статистическими константами.
func applyEquilibriumFeatures(f *domain.TimeframeFeatures, high, low, close []float64) {
	const window = 48
	if len(close) < window*2 {
		return
	}
	oldH, oldL := bounds(high[len(high)-window*2:len(high)-window], low[len(low)-window*2:len(low)-window])
	newH, newL := bounds(high[len(high)-window:], low[len(low)-window:])
	oldMid, newMid := (oldH+oldL)/2, (newH+newL)/2
	oldWidth, newWidth := oldH-oldL, newH-newL
	if oldMid > 0 {
		f.RollingMidDriftPct = 100 * (newMid - oldMid) / oldMid
	}
	if f.ATR > 0 {
		f.RollingMidDriftATR = (newMid - oldMid) / f.ATR
	}
	if newMid > 0 {
		f.RangeWidthPct = 100 * newWidth / newMid
	}
	if oldWidth > 0 {
		f.RangeWidthChangePct = 100 * (newWidth - oldWidth) / oldWidth
	}
	f.MidpointCrossings, f.MeanReversionRatio = meanReversion(close[len(close)-window:], newMid, newWidth)
}

// bounds возвращает high/low окна без зависимости от торгового направления.
func bounds(high, low []float64) (float64, float64) {
	hi, lo := high[0], low[0]
	for i := 1; i < len(high); i++ {
		if high[i] > hi {
			hi = high[i]
		}
		if low[i] < lo {
			lo = low[i]
		}
	}
	return hi, lo
}

// meanReversion считает пересечения midpoint и долю значимых excursions, вернувшихся к центру.
// Excursion считается значимым после удаления минимум на 20% ширины текущего range от midpoint.
func meanReversion(close []float64, midpoint, width float64) (float64, float64) {
	if len(close) < 2 || width <= 0 {
		return 0, 0
	}
	crossings, excursions, returns := 0, 0, 0
	threshold := width * 0.20
	state := 0
	for i := 1; i < len(close); i++ {
		prev, cur := close[i-1]-midpoint, close[i]-midpoint
		if (prev < 0 && cur >= 0) || (prev > 0 && cur <= 0) {
			crossings++
		}
		if state == 0 {
			if cur >= threshold {
				state = 1
				excursions++
			}
			if cur <= -threshold {
				state = -1
				excursions++
			}
		} else if (state == 1 && cur <= 0) || (state == -1 && cur >= 0) {
			returns++
			state = 0
		}
	}
	if excursions == 0 {
		return float64(crossings), 0
	}
	return float64(crossings), float64(returns) / float64(excursions)
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
	bu, be := []bool{bull(a), bull(b), bull(d)}, []bool{bear(a), bear(b), bear(d)}
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
	short, long := a, b
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
