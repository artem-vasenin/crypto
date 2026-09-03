// internal/execution/risk.go
package execution

import (
	"math"
	"strconv"
	"strings"

	"universal-bybit-screener/models"
)

func CalculateDynamicLeverage(c models.Candidate, targetStrategy string, maxLeverage int) int {
	if maxLeverage <= 1 {
		return 1
	}

	res, ok := c.Strategies[targetStrategy]
	if !ok || res.Score < 55.0 {
		return 1
	}

	scoreFactor := 1.0
	if res.Score < 75.0 {
		scoreFactor = 0.66
	}

	volatilityFactor := 1.0
	if c.Indicators.ATR1hPct > 3.5 {
		volatilityFactor = 0.33
	} else if c.Indicators.ATR1hPct > 2.0 {
		volatilityFactor = 0.66
	}

	liquidityFactor := 1.0
	if c.Market.Turnover24h < 2000000.0 {
		liquidityFactor = 0.5
	}

	calculatedLeverage := float64(maxLeverage) * scoreFactor * volatilityFactor * liquidityFactor
	finalLeverage := int(math.Floor(calculatedLeverage))

	if finalLeverage < 1 {
		return 1
	}
	if finalLeverage > maxLeverage {
		return maxLeverage
	}

	return finalLeverage
}

func CalculatePositionQty(marginUSD float64, leverage int, price, qtyStep, minQty, minNotional float64) float64 {
	if price <= 0 || leverage <= 0 || marginUSD <= 0 || qtyStep <= 0 {
		return 0
	}

	notionalUSD := marginUSD * float64(leverage)
	targetMinNotional := math.Max(minNotional, 10.5)

	if notionalUSD < targetMinNotional {
		return 0
	}

	rawQty := notionalUSD / price
	if rawQty < minQty {
		return 0
	}

	precision := GetPrecision(qtyStep)
	factor := math.Pow(10, float64(precision))
	qty := math.Floor((rawQty / qtyStep)) * qtyStep
	qty = math.Floor(qty*factor+0.5) / factor

	if qty < minQty {
		return 0
	}

	return qty
}

func RoundToStep(val, step float64) float64 {
	if step <= 0 || val <= 0 {
		return val
	}
	precision := GetPrecision(step)
	factor := math.Pow(10, float64(precision))
	return math.Round(math.Round(val/step)*step*factor) / factor
}

func GetPrecision(step float64) int {
	str := strconv.FormatFloat(step, 'f', -1, 64)
	parts := strings.Split(str, ".")
	if len(parts) < 2 {
		return 0
	}
	return len(parts[1])
}

func FormatStep(val, step float64) string {
	precision := GetPrecision(step)
	return strconv.FormatFloat(RoundToStep(val, step), 'f', precision, 64)
}

func ValidateStopLoss(side string, entryPrice, slPrice, maxRiskPct float64) bool {
	if slPrice <= 0 || entryPrice <= 0 {
		return false
	}

	distPct := 0.0
	if strings.EqualFold(side, "Buy") {
		if slPrice >= entryPrice {
			return false
		}
		distPct = (entryPrice - slPrice) / entryPrice * 100.0
	} else if strings.EqualFold(side, "Sell") {
		if slPrice <= entryPrice {
			return false
		}
		distPct = (slPrice - entryPrice) / entryPrice * 100.0
	} else {
		return false
	}

	// Порог подняли с 0.8% до 1.2% для защиты от рыночного шума и комиссий Taker
	return distPct >= 1.2 && distPct <= maxRiskPct
}

func ValidateTakeProfit(side string, entryPrice, tpPrice, minProfitPct float64) bool {
	if tpPrice <= 0 || entryPrice <= 0 {
		return false
	}

	distPct := 0.0
	if strings.EqualFold(side, "Buy") {
		if tpPrice <= entryPrice {
			return false
		}
		distPct = (tpPrice - entryPrice) / entryPrice * 100.0
	} else if strings.EqualFold(side, "Sell") {
		if tpPrice >= entryPrice {
			return false
		}
		distPct = (entryPrice - tpPrice) / entryPrice * 100.0
	} else {
		return false
	}

	effectiveMinProfit := math.Max(minProfitPct, 1.8)
	return distPct >= effectiveMinProfit && distPct <= 20.0
}
