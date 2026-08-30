package strategies

import "universal-bybit-screener/models"

// Long оценивает обычную направленную Long-позицию, а не Grid Bot.
// HH/HL — основа оценки (20+20 на 1h), 4h подтверждает контекст, а RSI/объём/funding
// имеют меньшие веса, чтобы единичный индикатор не переопределял структуру рынка.
type Long struct{}

func (Long) Name() string { return "long" }
func (Long) Evaluate(m models.MarketData, i models.Indicators, s map[string]models.Structure, l models.Levels) models.StrategyResult {
	score := 0.0
	st1, st4 := s["1h"], s["4h"]
	if st1.HighState == "HH" {
		score += 20
	}
	if st1.LowState == "HL" {
		score += 20
	}
	if st4.HighState == "HH" {
		score += 12
	}
	if st4.LowState == "HL" {
		score += 12
	}
	if i.RSI1h > 50 && i.RSI1h < 70 {
		score += 10
	} else if i.RSI1h >= 70 {
		score -= 5
	}
	if i.RSI4h > 50 && i.RSI4h < 70 {
		score += 8
	}
	if m.Ticker.Price24hPcnt > 0 {
		score += 5
	}
	if i.VolumeRatio1h > 1 {
		score += 6
	}
	if m.Ticker.FundingRate < 0 {
		score += 3
	}
	if st1.HighState == "LH" || st1.LowState == "LL" {
		score -= 15
	}
	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "directional trend + momentum + volume + derivatives"}
}
