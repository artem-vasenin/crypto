// internal/strategies/short.go
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

	// 1. HARD GATES (Каскадная защита)

	// Блокировка 1: Глобальный аптренд (1h + 4h)
	if st1.HighState == "HH" && st1.LowState == "HL" && st4.HighState == "HH" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "confirmed macro uptrend (1h+4h HH)"}
	}

	// Блокировка 2: Запрет шорта на растущем 15m импульсе
	if st15.HighState == "HH" && st15.LowState == "HL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "15m active micro-uptrend (HH+HL) - wait for pivot confirmation"}
	}

	// Блокировка 3: Отсутствие медвежьего разворотного триггера на 5m
	if st5.HighState == "HH" && st5.LowState == "HL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "5m micro pump active (no short trigger)"}
	}

	if c.Indicators.ATR1hPct > 4.0 || c.Indicators.ATR15m == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive/invalid volatility"}
	}

	if c.Market.SpreadPct > 0.08 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "spread exceeds 0.08% threshold"}
	}

	if c.OrderBook.ImbalancePct > -3.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient ask dominance in orderbook (imbalance > -3%)"}
	}

	if c.Derivatives.FundingRate < -0.0003 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "overheated negative funding rate (<-0.03%)"}
	}

	if c.Levels.RangePositionPct < 65.0 && c.Levels.NearestSupport > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry outside pullback zone (<65% range position)"}
	}

	if c.Indicators.RSI1h <= 35.0 || c.Indicators.RSI1h > 62.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h invalid for short pullback entry"}
	}

	priceDown := c.Market.Change24h < 0
	oiUp := c.Derivatives.OpenInterestChange > 0.25

	if !oiUp {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient Open Interest influx (<0.25%)"}
	}

	score := 0.0

	// 2. SCORING
	if priceDown {
		score += 10
	}

	if st5.HighState == "LH" || st5.LowState == "LL" {
		score += 15
	}
	if st15.HighState == "LH" || st15.LowState == "LL" {
		score += 15
	}
	if st1.HighState == "LH" || st1.LowState == "LL" {
		score += 20
	}
	if st4.HighState == "LH" || st4.LowState == "LL" {
		score += 10
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
		Reason: "multi-tf pullback confirmation (5m-4h) + OI expansion + L2 backing",
	}
}
