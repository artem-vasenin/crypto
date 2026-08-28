package strategies

import "sc/models"

// LongGrid ищет условия для Grid Bot с преимуществом нижней/восходящей стороны.
type LongGrid struct{}

func (LongGrid) Name() string { return "long-grid" }
func (LongGrid) Evaluate(m models.MarketData, i models.Indicators, s map[string]models.Structure, l models.Levels) models.StrategyResult {
	score := 0.0
	st := s["1h"]
	if st.HighState == "HH" {
		score += 15
	}
	if st.LowState == "HL" {
		score += 15
	}
	if st.HighState == "LH" {
		score -= 10
	}
	if st.LowState == "LL" {
		score -= 12
	}
	if i.ATR1hPct >= 1.5 {
		score += 10
	} else if i.ATR1hPct >= 1 {
		score += 5
	}
	if l.NearestSupport > 0 {
		dist := (m.Ticker.LastPrice - l.NearestSupport) / m.Ticker.LastPrice * 100
		if dist >= 0 && dist <= 5 {
			score += 12
		} else if dist <= 10 {
			score += 6
		}
	}
	if l.RangeWidthPct >= 5 {
		score += 8
	}
	if i.RSI1h >= 40 && i.RSI1h <= 65 {
		score += 8
	}
	if m.Ticker.Price24hPcnt > 0 {
		score += 5
	}
	if i.VolumeRatio1h > 1 {
		score += 5
	}
	if m.Ticker.FundingRate < 0 {
		score += 3
	}
	if m.OrderBook.ImbalancePct > 10 {
		score += 4
	} else if m.OrderBook.ImbalancePct < -40 {
		score -= 4
	}
	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "trend + volatility + support + grid range"}
}
