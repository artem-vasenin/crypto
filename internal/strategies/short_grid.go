package strategies

import "universal-bybit-screener/models"

type ShortGrid struct{}

func (ShortGrid) Name() string { return "short-grid" }

func (ShortGrid) Evaluate(c *models.Candidate) models.StrategyResult {
	st1h, ok1h := c.Structure["1h"]
	st15m, ok15m := c.Structure["15m"]

	// HARD GATES
	if c.Levels.NearestResistance <= 0 || c.Levels.NearestSupport <= 0 {
		return rejectShortGrid("missing support/resistance boundaries")
	}

	if !ok1h || st1h.HighState != "LH" {
		return rejectShortGrid("no LH confirmation on 1h (momentum still up)")
	}

	if c.Market.Change24h < 5 && c.Market.Change3d < 5 {
		return rejectShortGrid("insufficient bullish impulse")
	}

	if c.Market.Price <= 0 {
		return rejectShortGrid("invalid price")
	}

	resDistPct := (c.Levels.NearestResistance - c.Market.Price) / c.Market.Price * 100
	if resDistPct < 0 {
		return rejectShortGrid("price breached resistance")
	}
	if resDistPct > 5 {
		return rejectShortGrid("too far from resistance")
	}

	if c.Levels.RangePositionPct < 50 {
		return rejectShortGrid("entry too deep inside the range")
	}

	if c.Levels.RangeToATR1h < 2.0 {
		return rejectShortGrid("range too narrow vs ATR (high risk of breakout)")
	}

	// SCORING
	score := 0.0

	if c.Market.Change24h >= 15 {
		score += 15
	} else if c.Market.Change24h >= 8 {
		score += 10
	} else if c.Market.Change24h >= 5 {
		score += 5
	}

	if ok15m {
		if st15m.HighState == "LH" && st15m.LowState == "LL" {
			score += 15
		} else if st15m.HighState == "HH" {
			score -= 15
		}
	}

	switch {
	case resDistPct <= 1.5:
		score += 20
	case resDistPct <= 3.0:
		score += 10
	}

	if c.Indicators.VolumeRatio1h >= 1.5 {
		score += 5
	}

	if c.Market.SpreadPct <= 0.1 {
		score += 5
	}

	if c.Derivatives.FundingRate > 0.0001 {
		score += 10
	}

	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "exhausted impulse near resistance with confirmed local break"}
}

func rejectShortGrid(reason string) models.StrategyResult {
	return models.StrategyResult{Score: 0, Status: "reject", Reason: reason}
}
