// internal/strategies/long_grid.go
package strategies

import "universal-bybit-screener/models"

type LongGrid struct{}

func (LongGrid) Name() string { return "long-grid" }

func (LongGrid) Evaluate(c *models.Candidate) models.StrategyResult {
	if c.Levels.NearestSupport == 0 || c.Levels.NearestResistance == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "missing bounds for grid channel"}
	}

	// Hard Gate: Вход слишком близко к сопротивлению (> 75% высоты канала)
	if c.Levels.RangePositionPct > 75 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "entry too close to resistance"}
	}

	score := 40.0
	st1 := c.Structure["1h"]

	if st1.LowState == "HL" {
		score += 25
	}
	if c.Levels.RangePositionPct >= 15 && c.Levels.RangePositionPct <= 40 {
		score += 20 // Вход у нижней границы восходящего канала
	}
	if c.Levels.RangeToATR1h >= 2.0 {
		score += 15
	}

	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "ascending channel with safe entry"}
}
