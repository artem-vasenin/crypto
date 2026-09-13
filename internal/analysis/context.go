package analysis

import (
	"math"
	"sort"

	"universal-bybit-screener/models"
)

func buildMarketContext(structures map[string]models.Structure, currentPrice float64) models.MarketContext {
	st1 := structures["1h"]
	st4 := structures["4h"]

	bull1 := isBullish(st1)
	bear1 := isBearish(st1)
	bull4 := isBullish(st4)
	bear4 := isBearish(st4)

	ctx := models.MarketContext{}
	switch {
	case bull1 && bull4:
		ctx.Direction = "bullish"
	case bear1 && bear4:
		ctx.Direction = "bearish"
	case bull1 || bear1 || bull4 || bear4:
		ctx.Direction = "conflict"
	default:
		ctx.Direction = "neutral"
	}

	ctx.HTFConflict = isConflict(st1) || isConflict(st4) || (bull1 && bear4) || (bear1 && bull4)

	st30 := structures["30m"]
	if ctx.HTFConflict {
		ctx.Regime = "expansion"
	} else if ctx.Direction == "bullish" && isBullish(st30) {
		ctx.Regime = "trend"
	} else if ctx.Direction == "bearish" && isBearish(st30) {
		ctx.Regime = "trend"
	} else {
		ctx.Regime = "uncertain"
	}

	localHighs := make([]float64, 0, 15)
	for _, tf := range []string{"5m", "15m", "30m"} {
		for _, pivot := range structures[tf].Highs {
			if pivot.Price > currentPrice {
				localHighs = append(localHighs, pivot.Price)
			}
		}
	}
	sort.Float64s(localHighs)
	if len(localHighs) > 0 && currentPrice > 0 {
		ctx.LocalResistance = localHighs[0]
		ctx.DistanceToLocalResistancePct = (ctx.LocalResistance - currentPrice) / currentPrice * 100
		if ctx.DistanceToLocalResistancePct < 0 || math.IsNaN(ctx.DistanceToLocalResistancePct) {
			ctx.DistanceToLocalResistancePct = 0
		}
	}

	return ctx
}

func isBullish(st models.Structure) bool {
	return st.HighState == "HH" && st.LowState == "HL"
}

func isBearish(st models.Structure) bool {
	return st.HighState == "LH" && st.LowState == "LL"
}

func isConflict(st models.Structure) bool {
	return (st.HighState == "HH" && st.LowState == "LL") ||
		(st.HighState == "LH" && st.LowState == "HL")
}
