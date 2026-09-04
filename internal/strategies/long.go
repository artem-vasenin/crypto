// internal/strategies/long.go
package strategies

import "universal-bybit-screener/models"

type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES (Жесткая отсечка)
	if st1.HighState == "LH" && st1.LowState == "LL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed downtrend"}
	}

	if c.Indicators.ATR1hPct > 4.0 || c.Indicators.ATR15m == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive/invalid volatility"}
	}

	if c.OrderBook.ImbalancePct < -20.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "orderbook heavily ask-dominated"}
	}

	// HARD GATE: Покупка разрешена ТОЛЬКО в нижней трети канала (на ретесте/откате)
	if c.Levels.RangePositionPct > 35.0 && c.Levels.NearestResistance > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry outside pullback zone (>35% range position)"}
	}

	// HARD GATE: Запрет входа при аномальном выплеске объема (Volume Climax)
	if c.Indicators.VolumeRatio1h > 3.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "volume climax detected (possible Exhaustion)"}
	}

	// HARD GATE: Фильтр RSI (покупка строго в диапазоне 40-58, избегаем перегретости)
	if c.Indicators.RSI1h >= 58.0 || c.Indicators.RSI1h < 40.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h invalid for pullback entry"}
	}

	priceUp := c.Market.Change24h > 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	// Вход строго при наличии подтвержденного притока открытого интереса
	if !oiUp || c.Derivatives.OpenInterestChange < 1.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient Open Interest influx (<1.5%)"}
	}

	score := 0.0

	// 2. SCORING (Оценка качества отката)
	// Переменная oiUp гарантированно true благодаря Hard Gate выше.
	// Проверяем только направление движения 24h цены для подтверждения тренда.
	if priceUp {
		score += 30 // New Money (24h Price UP + Подтвержденный приток OI)
	}

	// Поддержка структуры
	if st1.HighState == "HH" || st1.LowState == "HL" {
		score += 25
	}
	if st4.HighState == "HH" || st4.LowState == "HL" {
		score += 15
	}

	// Идеальная геометрия отката к поддержке (10% - 25% высоты диапазона)
	if c.Levels.RangePositionPct >= 10.0 && c.Levels.RangePositionPct <= 25.0 {
		score += 20
	}

	if c.OrderBook.ImbalancePct > 10.0 {
		score += 10
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "pullback to support + OI expansion + structure backing",
	}
}
