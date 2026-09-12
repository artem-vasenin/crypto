package strategies

import "universal-bybit-screener/models"

type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	scores := evaluateDirectionalBlocks(c, 1)
	eligible, reasons := directionalEligibility(scores, "long")

	// A bullish short-term setup is not enough to override a contradictory
	// higher-timeframe structure. TrendQuality is deliberately the first gate.
	if c.Structure["1h"].HighState == "LH" && c.Structure["1h"].LowState == "LL" {
		eligible = false
		reasons = append(reasons, "long blocked: 1h structure is bearish (LH+LL)")
	}
	if c.Structure["4h"].HighState == "LH" && c.Structure["4h"].LowState == "LL" {
		eligible = false
		reasons = append(reasons, "long blocked: 4h structure is bearish (LH+LL)")
	}
	if c.Indicators.ATR1hPct > 5 || c.Market.SpreadPct > 0.15 {
		eligible = false
		reasons = append(reasons, "long blocked: extreme volatility or spread")
	}
	if c.Indicators.RSI1h >= 70 || c.Indicators.RSI1h < 30 {
		eligible = false
		reasons = append(reasons, "long blocked: 1h RSI outside 30..70")
	}

	status := "watch"
	if eligible {
		status = "consider"
	}
	return models.StrategyResult{
		Scores: scores,
		Decision: models.StrategyDecision{
			Eligible:        eligible,
			Priority:        []string{"trend_quality", "asset_quality", "entry_quality", "market_quality", "derivatives_quality"},
			BlockingReasons: reasons,
		},
		Status: status,
		Reason: decisionReason(eligible, reasons, "long"),
	}
}
