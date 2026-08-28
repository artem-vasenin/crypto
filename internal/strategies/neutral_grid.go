package strategies

import "sc/models"

// NeutralGrid ищет не направление, а пригодный диапазон для двусторонней сетки.
// Поэтому тренд штрафуется, а наличие двух границ, достаточная ширина диапазона
// и нахождение цены ближе к центру получают основной вес.
type NeutralGrid struct{}

func (NeutralGrid) Name() string { return "neutral-grid" }
func (NeutralGrid) Evaluate(m models.MarketData, i models.Indicators, s map[string]models.Structure, l models.Levels) models.StrategyResult {
	score := 0.0
	st1, st4 := s["1h"], s["4h"]
	// Смешанная или равновесная структура предпочтительнее сильного тренда.
	if st1.HighState == "EQ" || st1.LowState == "EQ" {
		score += 12
	}
	if st1.HighState == "HH" && st1.LowState == "HL" {
		score -= 15
	}
	if st1.HighState == "LH" && st1.LowState == "LL" {
		score -= 15
	}
	if (st4.HighState == "HH" && st4.LowState == "HL") || (st4.HighState == "LH" && st4.LowState == "LL") {
		score -= 10
	} else {
		score += 10
	}
	// Для Grid нужны обе стороны диапазона.
	if l.NearestSupport > 0 {
		score += 10
	}
	if l.NearestResistance > 0 {
		score += 10
	}
	// Ширина диапазона оценивается относительно ATR, а не только абсолютным процентом.
	if l.RangeToATR1h >= 8 {
		score += 20
	} else if l.RangeToATR1h >= 5 {
		score += 15
	} else if l.RangeToATR1h >= 3 {
		score += 8
	} else if l.RangeToATR1h > 0 {
		score -= 5
	}
	// Слишком узкий абсолютный диапазон может быть экономически бесполезен после комиссий.
	if l.RangeWidthPct >= 12 {
		score += 10
	} else if l.RangeWidthPct >= 7 {
		score += 7
	} else if l.RangeWidthPct >= 4 {
		score += 3
	} else if l.RangeWidthPct > 0 {
		score -= 5
	}
	// Центральная зона лучше края: при старте у resistance/support у сетки меньше запаса.
	if l.RangePositionPct >= 35 && l.RangePositionPct <= 65 {
		score += 10
	} else if l.RangePositionPct >= 20 && l.RangePositionPct <= 80 {
		score += 5
	} else if l.RangePositionPct > 0 {
		score -= 4
	}
	// Умеренный RSI дополнительно подтверждает отсутствие перегрева.
	if i.RSI1h >= 40 && i.RSI1h <= 60 {
		score += 5
	} else if i.RSI1h < 30 || i.RSI1h > 70 {
		score -= 4
	}
	// Волатильность нужна, но чрезмерный памп/дамп для neutral grid опасен.
	if i.ATR1hPct >= 1.0 && i.ATR1hPct <= 5.0 {
		score += 5
	} else if i.ATR1hPct > 8 {
		score -= 5
	}
	if i.VolumeRatio1h >= 0.7 && i.VolumeRatio1h <= 2.0 {
		score += 3
	}
	if m.Ticker.Price24hPcnt > -8 && m.Ticker.Price24hPcnt < 8 {
		score += 3
	} else if m.Ticker.Price24hPcnt > 20 || m.Ticker.Price24hPcnt < -20 {
		score -= 5
	}
	if m.OrderBook.BidNotional > 0 || m.OrderBook.AskNotional > 0 {
		if m.OrderBook.ImbalancePct > -25 && m.OrderBook.ImbalancePct < 25 {
			score += 3
		} else if m.OrderBook.ImbalancePct > 60 || m.OrderBook.ImbalancePct < -60 {
			score -= 4
		}
	}
	if m.Ticker.Bid1Price > 0 && m.Ticker.Ask1Price > 0 && m.Ticker.LastPrice > 0 {
		spread := (m.Ticker.Ask1Price - m.Ticker.Bid1Price) / m.Ticker.LastPrice * 100
		if spread < 0.15 {
			score += 2
		} else if spread > 0.5 {
			score -= 5
		}
	}
	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "range + flat structure + volatility + price position + liquidity"}
}
