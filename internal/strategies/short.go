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

	// Отбраковка при сильном бычьем стакане
	if c.OrderBook.ImbalancePct > 30.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "orderbook order imbalance heavily bid-dominated"}
	}

	priceDown := c.Market.Change24h < 0
	oiDown := c.Derivatives.OpenInterestChange < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	if priceDown && oiDown && c.Market.Change24h < -2 && c.Derivatives.OpenInterestChange < -2 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "long liquidation detected (Price DOWN, OI DOWN)"}
	}

	score := 0.0

	// 2. OI / PRICE MATRIX
	if priceDown && oiUp {
		score += 20 // Aggressive Shorts
	} else if !priceDown && oiDown {
		score -= 15
	}

	// 3. STRUCTURE
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

	if st1.HighState == "HH" || st1.LowState == "HL" {
		score -= 15
	}

	// 4. INDICATORS & L2 LIQUIDITY
	if c.Indicators.RSI1h < 48 && c.Indicators.RSI1h > 32 {
		score += 10
	} else if c.Indicators.RSI1h <= 30 {
		score -= 10
	}

	if c.Indicators.VolumeRatio1h > 1.2 {
		score += 10
	}

	if c.OrderBook.ImbalancePct < -20.0 {
		score += 10 // Продавцы плотно стоят в L2 стакане
	}

	if c.Derivatives.FundingRate > 0 {
		score += 5
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "directional short + L2 ask pressure + aggressive shorts",
	}
}
