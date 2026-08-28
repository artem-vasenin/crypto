package structure

import (
	"sort"
	"universal-bybit-screener/models"
)

// Analyze ищет локальные pivot high/low и сравнивает последние два экстремума.
// Это базовая структура рынка; позднее её можно усилить кластеризацией уровней.
func Analyze(c []models.Candle, leftRight, maxPivots int) models.Structure {
	var highs, lows []models.Pivot
	if leftRight < 1 || len(c) < leftRight*2+1 {
		return models.Structure{Highs: highs, Lows: lows}
	}
	for i := leftRight; i < len(c)-leftRight; i++ {
		hi, lo := true, true
		for j := i - leftRight; j <= i+leftRight; j++ {
			if j == i {
				continue
			}
			if c[j].High >= c[i].High {
				hi = false
			}
			if c[j].Low <= c[i].Low {
				lo = false
			}
		}
		if hi {
			highs = append(highs, models.Pivot{Time: c[i].Time, Price: c[i].High})
		}
		if lo {
			lows = append(lows, models.Pivot{Time: c[i].Time, Price: c[i].Low})
		}
	}
	if len(highs) > maxPivots {
		highs = highs[len(highs)-maxPivots:]
	}
	if len(lows) > maxPivots {
		lows = lows[len(lows)-maxPivots:]
	}
	s := models.Structure{Highs: highs, Lows: lows}
	if len(highs) >= 2 {
		s.PreviousHigh = highs[len(highs)-2].Price
		s.CurrentHigh = highs[len(highs)-1].Price
		s.HighState = highState(s.PreviousHigh, s.CurrentHigh)
	}
	if len(lows) >= 2 {
		s.PreviousLow = lows[len(lows)-2].Price
		s.CurrentLow = lows[len(lows)-1].Price
		s.LowState = lowState(s.PreviousLow, s.CurrentLow)
	}
	return s
}
func highState(prev, current float64) string {
	if current > prev {
		return "HH"
	}
	if current < prev {
		return "LH"
	}
	return "EQ"
}
func lowState(prev, current float64) string {
	if current > prev {
		return "HL"
	}
	if current < prev {
		return "LL"
	}
	return "EQ"
}

// Levels сортирует уровни от ближайшего к дальнему. В старой версии порядок
// зависел от порядка pivot'ов, поэтому Resistance[0] не всегда был ближайшим.
func Levels(s models.Structure, price float64) models.Levels {
	r := models.Levels{}
	for _, p := range s.Highs {
		if p.Price > price {
			r.Resistance = append(r.Resistance, p.Price)
		}
	}
	for _, p := range s.Lows {
		if p.Price < price {
			r.Support = append(r.Support, p.Price)
		}
	}
	sort.Slice(r.Resistance, func(i, j int) bool { return r.Resistance[i] < r.Resistance[j] })
	sort.Slice(r.Support, func(i, j int) bool { return r.Support[i] > r.Support[j] })
	if len(r.Resistance) > 0 {
		r.NearestResistance = r.Resistance[0]
	}
	if len(r.Support) > 0 {
		r.NearestSupport = r.Support[0]
	}
	if r.NearestSupport > 0 && r.NearestResistance > r.NearestSupport {
		width := r.NearestResistance - r.NearestSupport
		r.RangeWidthPct = width / r.NearestSupport * 100
		r.RangePositionPct = (price - r.NearestSupport) / width * 100
	}
	return r
}

// ApplyATR дополняет уровень относительной шириной диапазона.
func ApplyATR(l models.Levels, atr, price float64) models.Levels {
	if atr > 0 && price > 0 && l.NearestResistance > l.NearestSupport {
		l.RangeToATR1h = (l.NearestResistance - l.NearestSupport) / atr
	}
	return l
}
