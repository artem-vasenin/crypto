// internal/strategies/long.go
package strategies

import "universal-bybit-screener/models"

type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES
	if st1.HighState == "LH" && st1.LowState == "LL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed downtrend"}
	}

	// Отбраковка опасных аномалий волатильности
	if c.Indicators.ATR1hPct > 4.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "excessive 1h volatility (ATR > 4%)"}
	}

	// Отбраковка при отрицательном дисбалансе стакана (продавцы давят)
	if c.OrderBook.ImbalancePct < -30.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "orderbook order imbalance heavily ask-dominated"}
	}

	priceUp := c.Market.Change24h > 0
	oiDown := c.Derivatives.OpenInterestChange < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	if priceUp && oiDown && c.Market.Change24h > 2 && c.Derivatives.OpenInterestChange < -2 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "short covering detected (Price UP, OI DOWN)"}
	}

	score := 0.0

	// 2. OI / PRICE MATRIX
	if priceUp && oiUp {
		score += 20 // New Money
	} else if !priceUp && oiDown {
		score -= 15 // Long Liquidation
	}

	// 3. STRUCTURE
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

	if st1.HighState == "LH" || st1.LowState == "LL" {
		score -= 15
	}

	// 4. INDICATORS & L2 LIQUIDITY
	if c.Indicators.RSI1h > 52 && c.Indicators.RSI1h < 68 {
		score += 10
	} else if c.Indicators.RSI1h >= 70 {
		score -= 10
	}

	if c.Indicators.VolumeRatio1h > 1.2 {
		score += 10
	}

	if c.OrderBook.ImbalancePct > 20.0 {
		score += 10 // Покупатели доминируют в L2 стакане
	}

	if c.Derivatives.FundingRate < 0 {
		score += 5
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "directional long + L2 imbalance + new money",
	}
}
