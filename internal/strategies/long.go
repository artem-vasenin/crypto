// internal/strategies/long.go
package strategies

import (
	"universal-bybit-screener/models"
)

type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES (Жесткие блокировки)
	// Блокируем ТОЛЬКО при подтвержденном макро-даунтренде на 1h и 4h одновременно
	if st1.HighState == "LH" && st1.LowState == "LL" && st4.HighState == "LH" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "confirmed macro downtrend (1h+4h LH)"}
	}

	if c.Indicators.ATR1hPct > 4.0 || c.Indicators.ATR15m == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive/invalid volatility"}
	}

	// HARD GATE: Спред не должен превышать 0.08%
	if c.Market.SpreadPct > 0.08 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "spread exceeds 0.08% threshold"}
	}

	// HARD GATE: Доминирование Bids в L2 стакане (минимум +3.0%)
	if c.OrderBook.ImbalancePct < 3.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient bid dominance in orderbook (imbalance < +3%)"}
	}

	// HARD GATE: Запрет покупок при перегретом фандинге (>0.03%)
	if c.Derivatives.FundingRate > 0.0003 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "overheated funding rate (>0.03%)"}
	}

	// HARD GATE: Рабочий коридор отката — нижние 35% диапазона
	if c.Levels.RangePositionPct > 35.0 && c.Levels.NearestResistance > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry outside pullback zone (>35% range position)"}
	}

	// HARD GATE: Фильтр RSI (покупка строго 38-65)
	if c.Indicators.RSI1h >= 65.0 || c.Indicators.RSI1h < 38.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h invalid for pullback entry"}
	}

	priceUp := c.Market.Change24h > 0
	oiUp := c.Derivatives.OpenInterestChange > 0.25 // Реалистичный тиковый приток OI

	if !oiUp {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient Open Interest influx (<0.25%)"}
	}

	score := 0.0

	// 2. SCORING
	if priceUp {
		score += 20
	}

	if st1.HighState == "HH" || st1.LowState == "HL" {
		score += 30
	}
	if st4.HighState == "HH" || st4.LowState == "HL" {
		score += 20
	}

	if c.Levels.RangePositionPct >= 5.0 && c.Levels.RangePositionPct <= 25.0 {
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
