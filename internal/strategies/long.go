package strategies

import "universal-bybit-screener/models"

// Long оценивает направленную Long-позицию с использованием матрицы Price/OI.
type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES
	if st1.HighState == "LH" && st1.LowState == "LL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "1h confirmed downtrend"}
	}

	priceUp := c.Market.Change24h > 0
	oiDown := c.Derivatives.OpenInterestChange < 0
	oiUp := c.Derivatives.OpenInterestChange > 0

	// Short Covering (Шорт-сквиз): цена растет, OI падает. Искусственный рост на ликвидациях.
	if priceUp && oiDown && c.Market.Change24h > 2 && c.Derivatives.OpenInterestChange < -2 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "short covering detected (Price UP, OI DOWN)"}
	}

	score := 0.0

	// 2. OI / PRICE MATRIX SCORING
	if priceUp && oiUp {
		score += 15 // New Money (агрессивный набор лонгов)
	} else if !priceUp && oiDown {
		score -= 10 // Long Liquidation (высадка лонгистов, ловить ножи опасно)
	}

	// 3. STRUCTURE SCORING
	if st1.HighState == "HH" {
		score += 20
	}
	if st1.LowState == "HL" {
		score += 20
	}
	if st4.HighState == "HH" {
		score += 12
	}
	if st4.LowState == "HL" {
		score += 12
	}

	if st1.HighState == "LH" || st1.LowState == "LL" {
		score -= 15
	}

	// 4. INDICATORS & LIQUIDITY
	if c.Indicators.RSI1h > 50 && c.Indicators.RSI1h < 70 {
		score += 10
	} else if c.Indicators.RSI1h >= 70 {
		score -= 5 // Перекупленность
	}

	if c.Indicators.RSI4h > 50 && c.Indicators.RSI4h < 70 {
		score += 8
	}

	if c.Indicators.VolumeRatio1h > 1 {
		score += 6
	}

	if c.Derivatives.FundingRate < 0 {
		score += 3 // Шорты платят лонгам
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "directional long + new money (OI/Price matrix)",
	}
}
