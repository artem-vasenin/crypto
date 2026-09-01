package strategies

import "universal-bybit-screener/models"

type NeutralGrid struct{}

func (NeutralGrid) Name() string { return "neutral-grid" }

func (NeutralGrid) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// HARD REJECT
	if c.Levels.NearestSupport <= 0 || c.Levels.NearestResistance <= 0 {
		return rejectNeutralGrid("no complete support/resistance range")
	}

	if isBroadeningWedge(st1) || isBroadeningWedge(st4) {
		return rejectNeutralGrid("broadening formation (HH+LL) detected - high risk of IL")
	}

	if isBullishStructure(st1) && isBullishStructure(st4) {
		return rejectNeutralGrid("strong bullish trend on 1h and 4h")
	}
	if isBearishStructure(st1) && isBearishStructure(st4) {
		return rejectNeutralGrid("strong bearish trend on 1h and 4h")
	}

	if c.Indicators.ATR4hPct > 0 && c.Levels.RangeWidthPct < (c.Indicators.ATR4hPct*1.5) {
		return rejectNeutralGrid("range width insufficient against 4h volatility")
	}

	if c.Levels.RangeToATR1h > 0 && c.Levels.RangeToATR1h < 2.5 {
		return rejectNeutralGrid("range is too narrow relative to ATR")
	}

	if abs(c.Market.Change24h) > 12 {
		return rejectNeutralGrid("24h price movement is too strong")
	}

	if c.Indicators.RSI1h > 70 || c.Indicators.RSI1h < 30 {
		return rejectNeutralGrid("momentum is overbought/oversold")
	}

	score := 50.0

	// Позиционирование
	pos := c.Levels.RangePositionPct
	if pos >= 40 && pos <= 60 {
		score += 20
	} else if pos >= 25 && pos <= 75 {
		score += 5
	} else {
		score -= 20
	}

	if st1.HighState == "EQ" || st1.LowState == "EQ" {
		score += 15
	}

	if c.Levels.RangeToATR1h >= 4 && c.Levels.RangeToATR1h <= 8 {
		score += 15
	}

	score = clamp(score)
	return models.StrategyResult{Score: score, Status: status(score), Reason: "stable range with multi-TF volatility protection"}
}

func rejectNeutralGrid(reason string) models.StrategyResult {
	return models.StrategyResult{Score: 0, Status: "reject", Reason: reason}
}

func isBullishStructure(st models.Structure) bool {
	return st.HighState == "HH" && st.LowState == "HL"
}

func isBearishStructure(st models.Structure) bool {
	return st.HighState == "LH" && st.LowState == "LL"
}

func isBroadeningWedge(st models.Structure) bool {
	return st.HighState == "HH" && st.LowState == "LL"
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
