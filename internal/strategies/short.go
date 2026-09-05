// internal/strategies/short.go
package strategies

import "universal-bybit-screener/models"

type Short struct{}

func (Short) Name() string { return "short" }

func (Short) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES (Жесткие блокировки)
	if st1.HighState == "HH" && st1.LowState == "HL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed uptrend (HH+HL)"}
	}

	if c.Indicators.ATR1hPct > 4.0 || c.Indicators.ATR15m == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive/invalid volatility"}
	}

	// HARD GATE: Спред не должен превышать 0.08%
	if c.Market.SpreadPct > 0.08 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "spread exceeds 0.08% threshold"}
	}

	// HARD GATE: Запрет продаж при доминировании покупателей в стакане
	if c.OrderBook.ImbalancePct > 15.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "orderbook heavily bid-dominated (imbalance > +15%)"}
	}

	// HARD GATE: Продажа разрешена ТОЛЬКО в верхней трети канала
	if c.Levels.RangePositionPct < 65.0 && c.Levels.NearestSupport > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry outside pullback zone (<65% range position)"}
	}

	// HARD GATE: Фильтр RSI (продажа в диапазоне 35-62)
	if c.Indicators.RSI1h <= 35.0 || c.Indicators.RSI1h > 62.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h invalid for short pullback entry"}
	}

	priceDown := c.Market.Change24h < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	if !oiUp || c.Derivatives.OpenInterestChange < 1.2 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient Open Interest influx (<1.2%)"}
	}

	score := 0.0

	// 2. SCORING
	if priceDown {
		score += 30
	}

	if st1.HighState == "LH" || st1.LowState == "LL" {
		score += 25
	}
	if st4.HighState == "LH" || st4.LowState == "LL" {
		score += 15
	}

	if c.Levels.RangePositionPct >= 75.0 && c.Levels.RangePositionPct <= 95.0 {
		score += 20
	}

	if c.OrderBook.ImbalancePct < -10.0 {
		score += 10
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "pullback to resistance + aggressive short OI expansion + structure backing",
	}
}
