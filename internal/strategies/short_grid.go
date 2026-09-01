// internal/strategies/short_grid.go
package strategies

import "universal-bybit-screener/models"

type ShortGrid struct{}

func (ShortGrid) Name() string { return "short-grid" }

func (ShortGrid) Evaluate(c *models.Candidate) models.StrategyResult {
	if c.Levels.NearestSupport == 0 || c.Levels.NearestResistance == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "missing support/resistance boundaries"}
	}

	// Hard Gate: Вход слишком близко к поддержке (< 25% высоты канала)
	if c.Levels.RangePositionPct < 25 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry too deep inside the range"}
	}

	score := 40.0
	st1 := c.Structure["1h"]

	if st1.HighState == "LH" {
		score += 25
	}
	if c.Levels.RangePositionPct >= 60 && c.Levels.RangePositionPct <= 85 {
		score += 20 // Вход у верхней границы нисходящего канала
	}
	if c.Levels.RangeToATR1h >= 2.0 {
		score += 15
	}

	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "descending channel near resistance zone"}
}
