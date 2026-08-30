package strategies

import "universal-bybit-screener/models"

// ShortGrid ищет импульс вверх, который замедляется возле сопротивления.
// Веса отражают приоритет признаков: структура/сопротивление/волатильность важнее
// вторичных подтверждений, а сильный HH+HL получает крупный штраф как главный риск.
type ShortGrid struct{}

func (ShortGrid) Name() string { return "short-grid" }
func (ShortGrid) Evaluate(m models.MarketData, i models.Indicators, s map[string]models.Structure, l models.Levels) models.StrategyResult {
	score := 0.0
	if m.Ticker.Price24hPcnt >= 20 {
		score += 10
	} else if m.Ticker.Price24hPcnt >= 10 {
		score += 6
	}
	if i.ATR1hPct >= 3 {
		score += 10
	} else if i.ATR1hPct >= 1.5 {
		score += 6
	}
	if i.RSI1h >= 80 {
		score += 7
	} else if i.RSI1h >= 70 {
		score += 5
	}
	st := s["1h"]
	if st.HighState == "LH" {
		score += 10
	}
	if st.LowState == "LL" {
		score += 6
	}
	if l.NearestResistance > 0 {
		dist := (l.NearestResistance - m.Ticker.LastPrice) / m.Ticker.LastPrice * 100
		if dist >= 0 && dist <= 5 {
			score += 10
		} else if dist <= 10 {
			score += 5
		}
	}
	if i.VolumeRatio1h > 1.5 {
		score += 4
	}
	if i.VolumeTrend1h > 1.2 {
		score += 3
	}
	if len(m.Funding) > 0 && m.Funding[0].Rate > 0 {
		score += 2
	}
	if m.Ticker.FundingRate > 0 {
		score += 4
	}
	if len(m.OpenInterest) >= 2 && m.OpenInterest[len(m.OpenInterest)-1].OpenInterest < m.OpenInterest[0].OpenInterest {
		score += 5
	}
	if m.OrderBook.ImbalancePct < -10 {
		score += 4
	} else if m.OrderBook.ImbalancePct > 40 {
		score -= 4
	}
	if m.Ticker.LastPrice > 0 && m.Ticker.Bid1Price > 0 && m.Ticker.Ask1Price > 0 {
		spread := (m.Ticker.Ask1Price - m.Ticker.Bid1Price) / m.Ticker.LastPrice * 100
		if spread < 0.15 {
			score += 5
		} else if spread > 0.5 {
			score -= 8
		}
	}
	if st.HighState == "HH" && st.LowState == "HL" {
		score -= 15
	}
	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "impulse + volatility + structure + resistance + derivatives"}
}
