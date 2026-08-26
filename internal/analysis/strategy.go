package analysis

import (
	"fmt"
	"math"

	"bybit-screener/internal/indicators"
)

type StrategyResult struct {
	Score  int    `json:"score"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func status(score int) string {
	switch {
	case score >= 75:
		return "consider"
	case score >= 55:
		return "watch"
	case score >= 40:
		return "risky"
	default:
		return "avoid"
	}
}

// Score is intentionally simple and transparent. It is a filter/heuristic,
// not a machine-learning model. The richer JSON is the primary product.
func Score(price, change24, change3d float64, rsi15, rsi1, rsi4 float64, atr15, atr1, atr4 float64, volumeRatio float64, s15, s1, s4 indicators.Structure, resistance, support []float64, funding, fundingAvg, oiChange float64, bookImbalance, tradeImbalance float64) map[string]StrategyResult {
	short := 0
	long := 0
	shortGrid := 0
	longGrid := 0

	if change24 > 8 {
		short += 10
		shortGrid += 10
	}
	if change3d > 20 {
		short += 10
		shortGrid += 10
	}
	if rsi1 > 65 {
		short += 12
		shortGrid += 8
	}
	if rsi4 > 65 {
		short += 8
		shortGrid += 8
	}
	if s1.HighState == "LH" || s1.LowState == "LL" {
		short += 12
	}
	if s15.HighState == "LH" || s15.LowState == "LL" {
		short += 8
	}
	if len(resistance) > 0 {
		shortGrid += 10
	}
	if funding > fundingAvg && funding > 0 {
		shortGrid += 7
	}
	if oiChange < 0 {
		shortGrid += 7
	}
	if bookImbalance < -0.1 {
		short += 5
		shortGrid += 5
	}
	if tradeImbalance < -0.1 {
		short += 5
		shortGrid += 5
	}
	if volumeRatio < 0.7 && change24 > 5 {
		short += 5
		shortGrid += 5
	}
	if atr1 > 0 && price > 0 && atr1/price*100 > 2 {
		shortGrid += 8
	}

	if change24 < -3 {
		long += 8
		longGrid += 8
	}
	if change3d < -10 {
		long += 10
		longGrid += 8
	}
	if rsi1 < 35 {
		long += 12
		longGrid += 8
	}
	if rsi4 < 35 {
		long += 8
		longGrid += 8
	}
	if s1.HighState == "HH" || s1.LowState == "HL" {
		long += 10
	}
	if s15.HighState == "HH" || s15.LowState == "HL" {
		long += 8
	}
	if len(support) > 0 {
		longGrid += 10
	}
	if funding < 0 {
		longGrid += 5
	}
	if oiChange < 0 && change24 < 0 {
		longGrid += 5
	}
	if bookImbalance > 0.1 {
		long += 5
		longGrid += 5
	}
	if tradeImbalance > 0.1 {
		long += 5
		longGrid += 5
	}
	if atr1 > 0 && price > 0 && atr1/price*100 > 2 {
		longGrid += 8
	}

	// Small penalty for extreme spread/weak activity is applied by the caller
	// through the market filter; the scores themselves stay interpretable.
	_ = atr15
	_ = atr4
	_ = s4

	clamp := func(v int) int { return int(math.Max(0, math.Min(100, float64(v)))) }
	short, shortGrid, long, longGrid = clamp(short), clamp(shortGrid), clamp(long), clamp(longGrid)

	return map[string]StrategyResult{
		"long":       {Score: long, Status: status(long), Reason: fmt.Sprintf("trend + momentum + volume")},
		"long-grid":  {Score: longGrid, Status: status(longGrid), Reason: "trend + volatility + support"},
		"short":      {Score: short, Status: status(short), Reason: "trend reversal and momentum confirmation"},
		"short-grid": {Score: shortGrid, Status: status(shortGrid), Reason: "impulse + volatility + structure + resistance + derivatives"},
	}
}
