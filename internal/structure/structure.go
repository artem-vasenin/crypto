// internal/structure/structure.go
package structure

import (
	"sort"
	"universal-bybit-screener/models"
)

func Analyze(candles []models.Candle, leftRight, maxPivots int) models.Structure {
	var highs, lows []models.Pivot
	n := len(candles)

	if leftRight < 1 || n < leftRight*2+1 {
		return models.Structure{}
	}

	for i := leftRight; i < n-leftRight; i++ {
		isHigh, isLow := true, true
		for j := i - leftRight; j <= i+leftRight; j++ {
			if j == i {
				continue
			}
			if candles[j].High >= candles[i].High {
				isHigh = false
			}
			if candles[j].Low <= candles[i].Low {
				isLow = false
			}
		}

		if isHigh {
			highs = append(highs, models.Pivot{Time: candles[i].Time, Price: candles[i].High})
		}
		if isLow {
			lows = append(lows, models.Pivot{Time: candles[i].Time, Price: candles[i].Low})
		}
	}

	if len(highs) > maxPivots {
		highs = highs[len(highs)-maxPivots:]
	}
	if len(lows) > maxPivots {
		lows = lows[len(lows)-maxPivots:]
	}

	res := models.Structure{
		Highs: highs,
		Lows:  lows,
	}

	if len(highs) >= 2 {
		res.PreviousHigh = highs[len(highs)-2].Price
		res.CurrentHigh = highs[len(highs)-1].Price
		res.HighState = fetchHighState(res.PreviousHigh, res.CurrentHigh)
	}

	if len(lows) >= 2 {
		res.PreviousLow = lows[len(lows)-2].Price
		res.CurrentLow = lows[len(lows)-1].Price
		res.LowState = fetchLowState(res.PreviousLow, res.CurrentLow)
	}

	return res
}

func fetchHighState(prev, curr float64) string {
	if curr > prev {
		return "HH"
	}
	if curr < prev {
		return "LH"
	}
	return "EQ"
}

func fetchLowState(prev, curr float64) string {
	if curr > prev {
		return "HL"
	}
	if curr < prev {
		return "LL"
	}
	return "EQ"
}

func Levels(s models.Structure, currentPrice float64) models.Levels {
	lvl := models.Levels{}

	for _, p := range s.Highs {
		if p.Price > currentPrice {
			lvl.Resistance = append(lvl.Resistance, p.Price)
		}
	}
	for _, p := range s.Lows {
		if p.Price < currentPrice {
			lvl.Support = append(lvl.Support, p.Price)
		}
	}

	sort.Slice(lvl.Resistance, func(i, j int) bool {
		return lvl.Resistance[i] < lvl.Resistance[j]
	})

	sort.Slice(lvl.Support, func(i, j int) bool {
		return lvl.Support[i] > lvl.Support[j]
	})

	if len(lvl.Resistance) > 0 {
		lvl.NearestResistance = lvl.Resistance[0]
	}
	if len(lvl.Support) > 0 {
		lvl.NearestSupport = lvl.Support[0]
	}

	if lvl.NearestSupport > 0 && lvl.NearestResistance > lvl.NearestSupport {
		width := lvl.NearestResistance - lvl.NearestSupport
		lvl.RangeWidthPct = (width / lvl.NearestSupport) * 100
		lvl.RangePositionPct = ((currentPrice - lvl.NearestSupport) / width) * 100
	}

	return lvl
}

func ApplyATR(lvl models.Levels, atr1h, currentPrice float64) models.Levels {
	if atr1h > 0 && currentPrice > 0 && lvl.NearestResistance > lvl.NearestSupport {
		lvl.RangeToATR1h = (lvl.NearestResistance - lvl.NearestSupport) / atr1h
	}
	return lvl
}
