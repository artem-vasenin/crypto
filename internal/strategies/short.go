package strategies

import "universal-bybit-screener/models"

// Short оценивает обычную направленную Short-позицию.
type Short struct{}

func (Short) Name() string { return "short" }
func (Short) Evaluate(m models.MarketData, i models.Indicators, s map[string]models.Structure, l models.Levels) models.StrategyResult {
	score := 0.0
	st1, st4 := s["1h"], s["4h"]
	if st1.HighState == "LH" {
		score += 20
	}
	if st1.LowState == "LL" {
		score += 20
	}
	if st4.HighState == "LH" {
		score += 12
	}
	if st4.LowState == "LL" {
		score += 12
	}
	if i.RSI1h < 50 && i.RSI1h > 30 {
		score += 10
	} else if i.RSI1h <= 30 {
		score -= 5
	}
	if i.RSI4h < 50 && i.RSI4h > 30 {
		score += 8
	}
	if m.Ticker.Price24hPcnt < 0 {
		score += 5
	}
	if i.VolumeRatio1h > 1 {
		score += 6
	}
	if m.Ticker.FundingRate > 0 {
		score += 3
	}
	if st1.HighState == "HH" || st1.LowState == "HL" {
		score -= 15
	}
	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "directional downtrend + momentum + volume + derivatives"}
}
