package strategies

import (
	"universal-bybit-screener/models"
)

type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	st5 := c.Structure["5m"]
	st15 := c.Structure["15m"]
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// 1. HARD GATES (Исправлены завышенные пороги)

	// Запрет Long при явном даунтренде старших TF
	if st1.HighState == "LH" && st1.LowState == "LL" && st4.HighState == "LH" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "macro downtrend 1h+4h"}
	}

	// Фильтр падающего ножа: запрет если 5m совершает импульсный пробой вниз
	if st5.HighState == "LH" && st5.LowState == "LL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "5m active falling knife"}
	}

	if c.Indicators.ATR1hPct > 5.0 || c.Indicators.ATR15m == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "invalid volatility"}
	}

	if c.Market.SpreadPct > 0.15 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "spread > 0.15%"}
	}

	if c.OrderBook.ImbalancePct < 1.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "no bid imbalance in orderbook (<1.5%)"}
	}

	if c.Derivatives.FundingRate > 0.0005 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "overheated funding (>0.05%)"}
	}

	// Расширен диапазон входа отката с 35% до 50%
	if c.Levels.RangePositionPct > 50.0 && c.Levels.NearestResistance > 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "price too close to resistance (>50% range)"}
	}

	if c.Indicators.RSI1h >= 70.0 || c.Indicators.RSI1h < 30.0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "RSI 1h out of range"}
	}

	// Смягчен порог притока Open Interest с 0.25% до 0.05%
	if c.Derivatives.OpenInterestChange < 0.05 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "insufficient OI expansion (<0.05%)"}
	}

	score := 0.0

	// 2. SCORING SYSTEM
	if c.Market.Change24h > 0 {
		score += 15
	}
	if st5.HighState == "HH" || st5.LowState == "HL" {
		score += 20
	}
	if st15.HighState == "HH" || st15.LowState == "HL" {
		score += 20
	}
	if st1.HighState == "HH" || st1.LowState == "HL" {
		score += 25
	}
	if c.Levels.RangePositionPct >= 5.0 && c.Levels.RangePositionPct <= 35.0 {
		score += 20
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "long candidate passed filtered criteria",
	}
}
