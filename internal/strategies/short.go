// internal/strategies/short.go
package strategies

import "universal-bybit-screener/models"

// Short оценивает направленную Short-позицию с использованием матрицы Price/OI
type Short struct{}

func (Short) Name() string { return "short" }

func (Short) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES: Забраковка при подтвержденном аптренде 1h
	if st1.HighState == "HH" && st1.LowState == "HL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed uptrend"}
	}

	priceDown := c.Market.Change24h < 0
	oiDown := c.Derivatives.OpenInterestChange < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	// Hard Gate: Long Liquidation (каскад стопов лонгов) — высок риск V-образного отскока
	if priceDown && oiDown && c.Market.Change24h < -2 && c.Derivatives.OpenInterestChange < -2 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "long liquidation detected (Price DOWN, OI DOWN)"}
	}

	score := 0.0

	// 2. OI / PRICE MATRIX SCORING
	if priceDown && oiUp {
		score += 15 // Aggressive Shorts (крупный продавец давит по рынку)
	} else if !priceDown && oiDown {
		score -= 10 // Short Covering
	}

	// 3. STRUCTURE SCORING
	if st1.HighState == "LH" {
		score += 20
	}
	if st1.LowState == "LL" {
		score += 20
	}
	if st4.HighState == "LH" {
		score += 12
	}
	if st4.LowState == "LL" {
		score += 12
	}

	if st1.HighState == "HH" || st1.LowState == "HL" {
		score -= 15
	}

	// 4. INDICATORS & LIQUIDITY
	if c.Indicators.RSI1h < 50 && c.Indicators.RSI1h > 30 {
		score += 10
	} else if c.Indicators.RSI1h <= 30 {
		score -= 5 // Зона перепроданности
	}

	if c.Indicators.RSI4h < 50 && c.Indicators.RSI4h > 30 {
		score += 8
	}

	if c.Indicators.VolumeRatio1h > 1.0 {
		score += 6
	}

	if c.Derivatives.FundingRate > 0 {
		score += 3 // Лонги платят шортам
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "directional short + aggressive shorting (OI/Price matrix)",
	}
}
