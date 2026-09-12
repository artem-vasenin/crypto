package strategies

import "universal-bybit-screener/models"

type Short struct{}

func (Short) Name() string { return "short" }

func (Short) Evaluate(c *models.Candidate) models.StrategyResult {
	scores := evaluateDirectionalBlocks(c, -1)
	eligible, reasons := directionalEligibility(scores, "short")

	if c.Structure["1h"].HighState == "HH" && c.Structure["1h"].LowState == "HL" {
		eligible = false
		reasons = append(reasons, "short blocked: 1h structure is bullish (HH+HL)")
	}
	if c.Structure["4h"].HighState == "HH" && c.Structure["4h"].LowState == "HL" {
		eligible = false
		reasons = append(reasons, "short blocked: 4h structure is bullish (HH+HL)")
	}
	if c.Indicators.ATR1hPct > 5 || c.Market.SpreadPct > 0.15 {
		eligible = false
		reasons = append(reasons, "short blocked: extreme volatility or spread")
	}
	if c.Indicators.RSI1h <= 30 || c.Indicators.RSI1h > 70 {
		eligible = false
		reasons = append(reasons, "short blocked: 1h RSI outside 30..70")
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
		Reason: decisionReason(eligible, reasons, "short"),
	}
}
