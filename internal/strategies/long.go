package strategies

import (
	"fmt"
	"math"

	"universal-bybit-screener/models"
)

type Long struct{}

func (Long) Name() string { return "long" }

func (Long) Evaluate(c *models.Candidate) models.StrategyResult {
	scores := evaluateDirectionalBlocks(c, 1)
	eligible, reasons := directionalEligibility(scores, "long")

	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	if isBearish(st1) {
		eligible = false
		reasons = append(reasons, "long blocked: 1h structure is bearish (LH+LL)")
	}
	if isBearish(st4) {
		eligible = false
		reasons = append(reasons, "long blocked: 4h structure is bearish (LH+LL)")
	}
	if isConflict(st1) {
		eligible = false
		reasons = append(reasons, "long blocked: 1h structure is conflicting (HH+LL/LH+HL)")
	}
	if isConflict(st4) {
		eligible = false
		reasons = append(reasons, "long blocked: 4h structure is conflicting (HH+LL/LH+HL)")
	}

	// A late long after a strong impulse is especially vulnerable to a normal
	// pullback. Do not enter when price is very close to a local resistance and
	// the 15m RSI is already stretched. This is a gate, not a score bonus.
	if c.Context.LocalResistance > 0 &&
		c.Context.DistanceToLocalResistancePct <= math.Max(0.5*c.Indicators.ATR1hPct, 0.25) &&
		c.Indicators.RSI15m >= 68 &&
		c.Market.Change24h >= 5 {
		eligible = false
		reasons = append(reasons, fmt.Sprintf("long blocked: late entry %.2f%% below local resistance with 15m RSI %.1f", c.Context.DistanceToLocalResistancePct, c.Indicators.RSI15m))
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

func isBullish(st models.Structure) bool {
	return st.HighState == "HH" && st.LowState == "HL"
}

func isBearish(st models.Structure) bool {
	return st.HighState == "LH" && st.LowState == "LL"
}
