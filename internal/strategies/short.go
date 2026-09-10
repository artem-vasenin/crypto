package strategies

import (
	"universal-bybit-screener/models"
)

type Short struct{}

func (Short) Name() string { return "short" }

func (Short) Evaluate(c *models.Candidate) models.StrategyResult {
	st5 := c.Structure["5m"]
	st15 := c.Structure["15m"]
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES

	// Запрет Short при макро-аптренде
	if st1.HighState == "HH" && st1.LowState == "HL" && st4.HighState == "HH" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "macro uptrend 1h+4h"}
	}

	// Фильтр вертикального пампа: запрет шорта на растущем 5m импульсе
	if st5.HighState == "HH" && st5.LowState == "HL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "5m active micro pump"}
	}

	if c.Indicators.ATR1hPct > 5.0 || c.Indicators.ATR15m == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "invalid volatility"}
	}

	if c.Market.SpreadPct > 0.15 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "spread > 0.15%"}
	}

	if c.OrderBook.ImbalancePct > -1.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "no ask imbalance (< -1.5%)"}
	}

	if c.Derivatives.FundingRate < -0.0005 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "negative funding overheated (<-0.05%)"}
	}

	if c.Levels.RangePositionPct < 50.0 && c.Levels.NearestSupport > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "price too close to support (<50% range)"}
	}

	if c.Indicators.RSI1h <= 30.0 || c.Indicators.RSI1h > 70.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h out of bounds"}
	}

	if c.Derivatives.OpenInterestChange < 0.05 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient OI expansion (<0.05%)"}
	}

	score := 0.0

	// 2. SCORING SYSTEM
	if c.Market.Change24h < 0 {
		score += 15
	}
	if st5.HighState == "LH" || st5.LowState == "LL" {
		score += 20
	}
	if st15.HighState == "LH" || st15.LowState == "LL" {
		score += 20
	}
	if st1.HighState == "LH" || st1.LowState == "LL" {
		score += 25
	}
	if c.Levels.RangePositionPct >= 65.0 && c.Levels.RangePositionPct <= 95.0 {
		score += 20
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "short candidate passed filtered criteria",
	}
}
