// internal/strategies/short.go
package strategies

import "universal-bybit-screener/models"

type Short struct{}

func (Short) Name() string { return "short" }

func (Short) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES (Жесткая отсечка)
	if st1.HighState == "HH" && st1.LowState == "HL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed uptrend"}
	}

	if c.Indicators.ATR1hPct > 4.0 || c.Indicators.ATR15m == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive/invalid volatility"}
	}

	// HARD GATE: Запрет продаж при доминировании покупателей в стакане (Bid-dominated)
	if c.OrderBook.ImbalancePct > 15.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "orderbook heavily bid-dominated (imbalance > +15%)"}
	}

	// HARD GATE: Продажа разрешена ТОЛЬКО в верхней трети канала (на откате к сопротивлению)
	if c.Levels.RangePositionPct < 65.0 && c.Levels.NearestSupport > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry outside pullback zone (<65% range position)"}
	}

	// HARD GATE: Запрет продаж при аномальном выплеске объема (Volume Climax / Exhaustion)
	if c.Indicators.VolumeRatio1h > 3.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "volume climax detected (possible Exhaustion)"}
	}

	// HARD GATE: Фильтр RSI (продажа строго в диапазоне 42-60, избегаем перепроданности на дне)
	if c.Indicators.RSI1h <= 42.0 || c.Indicators.RSI1h > 60.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h invalid for short pullback entry"}
	}

	priceDown := c.Market.Change24h < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	// Вход строго при наличии подтвержденного притока коротких позиций (Aggressive Shorts)
	if !oiUp || c.Derivatives.OpenInterestChange < 1.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient Open Interest influx (<1.5%)"}
	}

	score := 0.0

	// 2. SCORING (Оценка качества отката)
	if priceDown {
		score += 30 // Aggressive Shorts (24h Price DOWN + Подтвержденный приток OI)
	}

	// Поддержка нисходящей структуры
	if st1.HighState == "LH" || st1.LowState == "LL" {
		score += 25
	}
	if st4.HighState == "LH" || st4.LowState == "LL" {
		score += 15
	}

	// Идеальная геометрия отката к сопротивлению (75% - 90% высоты диапазона)
	if c.Levels.RangePositionPct >= 75.0 && c.Levels.RangePositionPct <= 90.0 {
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
