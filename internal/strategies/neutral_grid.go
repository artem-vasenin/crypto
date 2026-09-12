// internal/strategies/neutral_grid.go
package strategies

import "universal-bybit-screener/models"

// NeutralGrid оценивает флетовые боковики для спотовых/сеточных ботов
type NeutralGrid struct{}

func (NeutralGrid) Name() string { return "neutral-grid" }

func (NeutralGrid) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	isUpTrend := func(st models.Structure) bool {
		return st.HighState == "HH" && st.LowState == "HL"
	}
	isDownTrend := func(st models.Structure) bool {
		return st.HighState == "LH" && st.LowState == "LL"
	}

	if isUpTrend(st1) || isDownTrend(st1) || isUpTrend(st4) || isDownTrend(st4) {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "directional trend detected on 1h/4h"}
	}

	if st1.HighState == "HH" && st1.LowState == "LL" {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "broadening formation (HH+LL)"}
	}
	if st1.HighState == "LH" && st1.LowState == "HL" {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "contracting formation (LH+HL)"}
	}

	if c.Levels.NearestResistance == 0 || c.Levels.NearestSupport == 0 {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "no complete support/resistance range"}
	}
	if c.Indicators.ATR1hPct <= 0 || c.Indicators.ATR1hPct > 3.0 {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "1h volatility too high for neutral grid"}
	}
	if c.Levels.RangeToATR1h < 2.0 {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "range too narrow versus 1h ATR"}
	}
	if c.Levels.RangeWidthPct < 3 || c.Levels.RangeWidthPct > 15 {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "range width outside neutral-grid limits"}
	}
	if c.Levels.RangePositionPct < 25 || c.Levels.RangePositionPct > 75 {
		return models.StrategyResult{Scores: models.AnalysisScores{EntryQuality: blockScore(0, "grid gate failed")}, Decision: models.StrategyDecision{Eligible: false}, Status: "reject", Reason: "price too close to range edge"}
	}

	score := 50.0
	if c.Levels.RangePositionPct >= 40 && c.Levels.RangePositionPct <= 60 {
		score += 25
	} else {
		score += 10
	}
	if c.Indicators.RSI1h >= 40 && c.Indicators.RSI1h <= 60 {
		score += 15
	}
	if c.Indicators.VolumeTrend1h <= 10 {
		score += 10
	}

	score = clamp(score)
	return models.StrategyResult{
		Scores:   models.AnalysisScores{EntryQuality: blockScore(score, "neutral-grid suitability")},
		Decision: models.StrategyDecision{Eligible: status(score) == "consider"},
		Status:   status(score),
		Reason:   "low-volatility range with trend and edge protection",
	}
}
