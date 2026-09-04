// internal/strategies/short.go
package strategies

import "universal-bybit-screener/models"

type Short struct{}

func (Short) Name() string { return "short" }

func (Short) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES
	if st1.HighState == "HH" && st1.LowState == "HL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed uptrend"}
	}

	if c.Indicators.ATR1hPct > 4.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive 1h volatility (ATR > 4%)"}
	}

	if c.OrderBook.ImbalancePct > 25.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "orderbook order imbalance heavily bid-dominated"}
	}

	// HARD GATE: Продажа разрешена строго в верхней части диапазона (>= 40% высоты диапазона)
	if c.Levels.RangePositionPct < 40.0 && c.Levels.NearestSupport > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry too close to support zone (<40% range)"}
	}

	// HARD GATE: Перепроданность RSI 1h
	if c.Indicators.RSI1h <= 32.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h oversold (<= 32)"}
	}

	// HARD GATE: Selling Climax Filter (запрет продаж на дне дампа)
	if c.Market.Change24h < -12.0 && c.Indicators.VolumeRatio1h > 2.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "selling climax detected (24h Change < -12% & VolumeRatio > 2.5)"}
	}

	priceDown := c.Market.Change24h < 0
	oiDown := c.Derivatives.OpenInterestChange < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	if priceDown && oiDown && c.Market.Change24h < -2 && c.Derivatives.OpenInterestChange < -2 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "long liquidation detected (Price DOWN, OI DOWN)"}
	}

	score := 0.0

	// 2. OI / PRICE MATRIX SCORING
	if priceDown && oiUp {
		score += 25 // Aggressive Shorts
	} else if !priceDown && oiDown {
		score -= 15
	}

	// 3. STRUCTURE SCORING
	if st1.HighState == "LH" {
		score += 20
	}
	if st1.LowState == "LL" {
		score += 20
	}
	if st4.HighState == "LH" {
		score += 10
	}
	if st4.LowState == "LL" {
		score += 10
	}

	// 4. INDICATORS & POSITION WITHIN RANGE
	if c.Indicators.RSI1h < 50 && c.Indicators.RSI1h > 35 {
		score += 10
	}

	if c.Indicators.VolumeRatio1h > 1.1 {
		score += 10
	}

	// Бонус за вход в верхней части канала (60% - 85%)
	if c.Levels.RangePositionPct >= 60.0 && c.Levels.RangePositionPct <= 85.0 {
		score += 15
	}

	if c.OrderBook.ImbalancePct < -15.0 {
		score += 10
	}

	if c.Derivatives.FundingRate > 0 {
		score += 5
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "directional short + safe range position + aggressive shorts",
	}
}
