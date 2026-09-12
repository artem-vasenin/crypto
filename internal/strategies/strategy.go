package strategies

import (
	"fmt"
	"math"

	"universal-bybit-screener/models"
)

type Strategy interface {
	Name() string
	Evaluate(c *models.Candidate) models.StrategyResult
}

func Names() []string {
	return []string{
		"long",
		"short",
		"long-grid",
		"short-grid",
		"neutral-grid",
	}
}

func New(name string) (Strategy, error) {
	switch name {
	case "long":
		return Long{}, nil
	case "short":
		return Short{}, nil
	case "long-grid":
		return LongGrid{}, nil
	case "short-grid":
		return ShortGrid{}, nil
	case "neutral-grid":
		return NeutralGrid{}, nil
	default:
		return nil, fmt.Errorf("unknown strategy: %s", name)
	}
}

// scoreCard is intentionally split into independent blocks. There is no
// universal score: macro/asset context and entry timing are different questions.
func blockScore(score float64, reason string) models.BlockScore {
	score = clamp(score)
	return models.BlockScore{Score: score, Status: blockStatus(score), Reason: reason}
}

func blockStatus(score float64) string {
	switch {
	case score >= 75:
		return "strong"
	case score >= 60:
		return "acceptable"
	case score >= 45:
		return "weak"
	default:
		return "poor"
	}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return math.Round(v)
}

// status is kept for grid strategies, where a single grid suitability score
// is still meaningful because they do not open directional positions.
func status(score float64) string {
	switch {
	case score >= 75:
		return "consider"
	case score >= 55:
		return "watch"
	case score >= 35:
		return "risky"
	default:
		return "avoid"
	}
}

func structureDirection(st models.Structure) int {
	if st.HighState == "HH" && st.LowState == "HL" {
		return 1
	}
	if st.HighState == "LH" && st.LowState == "LL" {
		return -1
	}
	return 0
}

func directionalStructureScore(st models.Structure, side int) float64 {
	dir := structureDirection(st)
	switch {
	case dir == side:
		return 100
	case dir == 0:
		return 50
	default:
		return 0
	}
}

func directionalChangeScore(value float64, side int, strong, neutral float64) float64 {
	if side > 0 {
		switch {
		case value >= strong:
			return 100
		case value >= neutral:
			return 70
		case value >= -neutral:
			return 45
		default:
			return 10
		}
	}

	switch {
	case value <= -strong:
		return 100
	case value <= -neutral:
		return 70
	case value <= neutral:
		return 45
	default:
		return 10
	}
}

func weighted(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func evaluateDirectionalBlocks(c *models.Candidate, side int) models.AnalysisScores {
	st5 := c.Structure["5m"]
	st15 := c.Structure["15m"]
	st30 := c.Structure["30m"]
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	asset := weighted(
		directionalChangeScore(c.Market.Change24h, side, 5, 1),
		directionalChangeScore(c.Market.Change3d, side, 8, 2),
		directionalChangeScore(c.Market.Change7d, side, 10, 3),
		(sideRSI(c.Indicators.RSI1h, side)+sideRSI(c.Indicators.RSI4h, side))/2,
	)

	trend := weighted(
		directionalStructureScore(st1, side),
		directionalStructureScore(st4, side),
		directionalStructureScore(st30, side),
	)

	entryRange := 0.0
	if side > 0 {
		switch {
		case c.Levels.RangePositionPct >= 15 && c.Levels.RangePositionPct <= 40:
			entryRange = 100
		case c.Levels.RangePositionPct >= 10 && c.Levels.RangePositionPct <= 50:
			entryRange = 70
		default:
			entryRange = 25
		}
	} else {
		switch {
		case c.Levels.RangePositionPct >= 60 && c.Levels.RangePositionPct <= 85:
			entryRange = 100
		case c.Levels.RangePositionPct >= 50 && c.Levels.RangePositionPct <= 95:
			entryRange = 70
		default:
			entryRange = 25
		}
	}

	entry := weighted(
		directionalStructureScore(st15, side),
		directionalStructureScore(st5, side),
		directionalRSIScore(c.Indicators.RSI15m, side),
		directionalRSIScore(c.Indicators.RSI5m, side),
		entryRange,
	)

	volumeScore := 50.0
	if c.Indicators.VolumeRatio1h >= 0.8 {
		volumeScore = 85
	} else if c.Indicators.VolumeRatio1h >= 0.5 {
		volumeScore = 65
	} else if c.Indicators.VolumeRatio1h < 0.3 {
		volumeScore = 25
	}
	if c.Indicators.VolumeTrend1h < -60 {
		volumeScore -= 20
	}
	market := weighted(
		spreadScore(c.Market.SpreadPct),
		turnoverScore(c.Market.Turnover24h),
		clamp(volumeScore),
	)

	derivatives := weighted(
		fundingScore(c.Derivatives.FundingRate, side),
		oiScore(c.Derivatives.OpenInterestChange, side),
		orderBookScore(c.OrderBook.ImbalancePct, side),
	)

	return models.AnalysisScores{
		AssetQuality:       blockScore(asset, "24h/3d/7d trajectory and 1h/4h RSI"),
		TrendQuality:       blockScore(trend, "1h/4h/30m market structure"),
		EntryQuality:       blockScore(entry, "5m/15m structure, RSI and range location"),
		MarketQuality:      blockScore(market, "spread, turnover and 1h volume"),
		DerivativesQuality: blockScore(derivatives, "funding, open interest and order book"),
	}
}

func sideRSI(rsi float64, side int) float64 {
	if rsi <= 0 || rsi >= 100 {
		return 50
	}
	if side > 0 {
		switch {
		case rsi >= 50 && rsi <= 65:
			return 100
		case rsi >= 40 && rsi < 50:
			return 65
		case rsi > 65 && rsi < 70:
			return 65
		default:
			return 25
		}
	}
	switch {
	case rsi >= 35 && rsi <= 50:
		return 100
	case rsi > 50 && rsi <= 60:
		return 65
	case rsi > 30 && rsi < 35:
		return 65
	default:
		return 25
	}
}

func directionalRSIScore(rsi float64, side int) float64 {
	return sideRSI(rsi, side)
}

func spreadScore(spread float64) float64 {
	switch {
	case spread <= 0:
		return 0
	case spread <= 0.03:
		return 100
	case spread <= 0.06:
		return 80
	case spread <= 0.10:
		return 60
	case spread <= 0.15:
		return 35
	default:
		return 0
	}
}

func turnoverScore(turnover float64) float64 {
	switch {
	case turnover >= 50_000_000:
		return 100
	case turnover >= 20_000_000:
		return 85
	case turnover >= 10_000_000:
		return 70
	case turnover >= 5_000_000:
		return 55
	default:
		return 20
	}
}

func fundingScore(rate float64, side int) float64 {
	// Positive funding favors shorts and is a mild headwind for longs.
	if side > 0 {
		switch {
		case rate < -0.0001:
			return 85
		case rate <= 0.0001:
			return 75
		case rate <= 0.0003:
			return 55
		default:
			return 20
		}
	}
	switch {
	case rate > 0.0001:
		return 85
	case rate >= -0.0001:
		return 75
	case rate >= -0.0003:
		return 55
	default:
		return 20
	}
}

func oiScore(change float64, side int) float64 {
	if side > 0 {
		if change >= 0.5 {
			return 90
		}
		if change >= 0.05 {
			return 70
		}
		if change >= -0.5 {
			return 50
		}
		return 30
	}
	if change >= 0.5 {
		return 90
	}
	if change >= 0.05 {
		return 70
	}
	if change >= -0.5 {
		return 50
	}
	return 30
}

func orderBookScore(imbalance float64, side int) float64 {
	if side > 0 {
		switch {
		case imbalance >= 10:
			return 100
		case imbalance >= 3:
			return 80
		case imbalance >= 0:
			return 60
		case imbalance >= -5:
			return 35
		default:
			return 15
		}
	}
	switch {
	case imbalance <= -10:
		return 100
	case imbalance <= -3:
		return 80
	case imbalance <= 0:
		return 60
	case imbalance <= 5:
		return 35
	default:
		return 15
	}
}

func directionalEligibility(scores models.AnalysisScores, side string) (bool, []string) {
	const (
		minTrend       = 60.0
		minAsset       = 55.0
		minEntry       = 60.0
		minMarket      = 60.0
		minDerivatives = 40.0
	)

	reasons := make([]string, 0, 5)
	if scores.TrendQuality.Score < minTrend {
		reasons = append(reasons, fmt.Sprintf("%s blocked: trend quality %.0f < %.0f", side, scores.TrendQuality.Score, minTrend))
	}
	if scores.AssetQuality.Score < minAsset {
		reasons = append(reasons, fmt.Sprintf("%s blocked: asset quality %.0f < %.0f", side, scores.AssetQuality.Score, minAsset))
	}
	if scores.EntryQuality.Score < minEntry {
		reasons = append(reasons, fmt.Sprintf("%s blocked: entry quality %.0f < %.0f", side, scores.EntryQuality.Score, minEntry))
	}
	if scores.MarketQuality.Score < minMarket {
		reasons = append(reasons, fmt.Sprintf("%s blocked: market quality %.0f < %.0f", side, scores.MarketQuality.Score, minMarket))
	}
	if scores.DerivativesQuality.Score < minDerivatives {
		reasons = append(reasons, fmt.Sprintf("%s blocked: derivatives quality %.0f < %.0f", side, scores.DerivativesQuality.Score, minDerivatives))
	}
	return len(reasons) == 0, reasons
}

func decisionReason(eligible bool, reasons []string, side string) string {
	if eligible {
		return side + " passed all block gates"
	}
	if len(reasons) == 0 {
		return side + " rejected"
	}
	return reasons[0]
}
