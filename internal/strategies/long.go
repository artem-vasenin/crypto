// internal/strategies/long.go
package strategies

import "universal-bybit-screener/models"

type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES (Абсолютные блокировки)
	if st1.HighState == "LH" && st1.LowState == "LL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed downtrend"}
	}

	if c.Indicators.ATR1hPct > 4.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive 1h volatility (ATR > 4%)"}
	}

	if c.OrderBook.ImbalancePct < -25.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "orderbook order imbalance heavily ask-dominated"}
	}

	// HARD GATE: Покупка разрешена строго в нижней и средней части канала (<= 60% высоты диапазона)
	if c.Levels.RangePositionPct > 60.0 && c.Levels.NearestResistance > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry too close to resistance zone (>60% range)"}
	}

	// HARD GATE: Перекупленность RSI 1h
	if c.Indicators.RSI1h >= 68.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h overbought (>= 68)"}
	}

	// HARD GATE: Buying Climax Filter (запрет входа на кульминации пампа)
	if c.Market.Change24h > 12.0 && c.Indicators.VolumeRatio1h > 2.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "buying climax detected (24h Change > 12% & VolumeRatio > 2.5)"}
	}

	priceUp := c.Market.Change24h > 0
	oiDown := c.Derivatives.OpenInterestChange < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	if priceUp && oiDown && c.Market.Change24h > 2 && c.Derivatives.OpenInterestChange < -2 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "short covering detected (Price UP, OI DOWN)"}
	}

	score := 0.0

	// 2. OI / PRICE MATRIX SCORING
	if priceUp && oiUp {
		score += 25 // New Money (приток капитала)
	} else if !priceUp && oiDown {
		score -= 15 // Long Liquidation
	}

	// 3. STRUCTURE SCORING
	if st1.HighState == "HH" {
		score += 20
	}
	if st1.LowState == "HL" {
		score += 20
	}
	if st4.HighState == "HH" {
		score += 10
	}
	if st4.LowState == "HL" {
		score += 10
	}

	// 4. INDICATORS & POSITION WITHIN RANGE
	if c.Indicators.RSI1h > 50 && c.Indicators.RSI1h < 65 {
		score += 10
	}

	if c.Indicators.VolumeRatio1h > 1.1 {
		score += 10
	}

	// Бонус за качественный откат к поддержке (15% - 40% от канала)
	if c.Levels.RangePositionPct >= 15.0 && c.Levels.RangePositionPct <= 40.0 {
		score += 15
	}

	if c.OrderBook.ImbalancePct > 15.0 {
		score += 10
	}

	if c.Derivatives.FundingRate < 0 {
		score += 5
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "directional long + safe range position + new money",
	}
}
