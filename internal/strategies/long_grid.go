package strategies

import "universal-bybit-screener/models"

type LongGrid struct{}

func (LongGrid) Name() string { return "long-grid" }

func (LongGrid) Evaluate(c *models.Candidate) models.StrategyResult {
	st1h := c.Structure["1h"]
	st4h := c.Structure["4h"]

	// HARD GATES
	if c.Levels.NearestResistance <= 0 || c.Levels.NearestSupport <= 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "missing bounds for grid channel"}
	}

	if c.Levels.RangePositionPct > 80 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry too close to resistance"}
	}

	if c.Levels.RangeToATR1h < 1.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "range too narrow relative to ATR"}
	}

	if st4h.HighState == "LH" && st4h.LowState == "LL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "4h downtrend"}
	}

	score := 0.0

	// 1. Структура
	if st1h.HighState == "HH" && st1h.LowState == "HL" {
		score += 15
	} else if st1h.LowState == "LL" {
		score -= 15
	}

	if st4h.HighState == "HH" && st4h.LowState == "HL" {
		score += 10
	} else if st4h.HighState == "LH" {
		score -= 10
	}

	// 2. Волатильность и Динамика
	if c.Indicators.ATR1hPct >= 1.0 && c.Indicators.ATR1hPct <= 3.0 {
		score += 10
	} else if c.Indicators.ATR1hPct > 5.0 {
		score -= 15
	}

	if c.Market.Change24h >= 0 && c.Market.Change24h <= 5 {
		score += 10
	} else if c.Market.Change24h > 15 {
		score -= 20 // Экстремальный памп - риск отката
	}

	// 3. Позиционирование
	if c.Levels.RangePositionPct <= 30 {
		score += 15
	} else if c.Levels.RangePositionPct <= 50 {
		score += 5
	} else {
		score -= 10
	}

	// 4. Индикаторы
	if c.Indicators.RSI1h >= 45 && c.Indicators.RSI1h <= 60 {
		score += 5
	} else if c.Indicators.RSI1h > 70 {
		score -= 15
	}

	if c.Derivatives.FundingRate < 0 {
		score += 5
	} else if c.Derivatives.FundingRate > 0.0005 {
		score -= 10
	}

	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "ascending channel with safe entry and controlled momentum"}
}
