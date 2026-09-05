// internal/execution/risk.go
package execution

import (
	"math"
	"strconv"
	"strings"

	"universal-bybit-screener/models"
)

// CalculateDynamicStopLoss гарантирует отступ не менее k * ATR(1h) для защиты от шума
func CalculateDynamicStopLoss(side string, entryPrice, pivotLevel, atr1h float64, atrMultiplier float64, tickStep float64) float64 {
	if entryPrice <= 0 || atr1h <= 0 {
		return 0
	}

	if atrMultiplier < 1.2 {
		atrMultiplier = 1.5 // Оптимальный коэффициент фильтрации шума
	}

	minDistance := atr1h * atrMultiplier
	rawSL := 0.0

	if strings.EqualFold(side, "Buy") {
		pivotDistance := entryPrice - pivotLevel
		actualDistance := math.Max(pivotDistance, minDistance)
		rawSL = entryPrice - actualDistance
	} else if strings.EqualFold(side, "Sell") {
		pivotDistance := pivotLevel - entryPrice
		actualDistance := math.Max(pivotDistance, minDistance)
		rawSL = entryPrice + actualDistance
	} else {
		return 0
	}

	if tickStep > 0 {
		return RoundToStep(rawSL, tickStep)
	}
	return rawSL
}

// CalculateDynamicTakeProfit рассчитывает Take-Profit с привязкой к Risk/Reward
func CalculateDynamicTakeProfit(side string, entryPrice, slPrice, minRR float64, tickStep float64) float64 {
	if entryPrice <= 0 || slPrice <= 0 {
		return 0
	}

	if minRR < 1.2 {
		minRR = 1.5
	}

	slDistance := math.Abs(entryPrice - slPrice)
	tpDistance := slDistance * minRR
	rawTP := 0.0

	if strings.EqualFold(side, "Buy") {
		rawTP = entryPrice + tpDistance
	} else if strings.EqualFold(side, "Sell") {
		rawTP = entryPrice - tpDistance
	} else {
		return 0
	}

	if tickStep > 0 {
		return RoundToStep(rawTP, tickStep)
	}
	return rawTP
}

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

	targetMinNotional := math.Max(minNotional*1.05, 5.25)

	targetNotional := marginUSD * float64(leverage)
	if targetNotional < targetMinNotional {
		targetNotional = targetMinNotional
	}

	calculatedQty := targetNotional / price
	qty := RoundToStep(calculatedQty, qtyStep)

	if qty < minQty {
		qty = minQty
	}

	if (qty * price) < targetMinNotional {
		qty = RoundToStep((targetMinNotional/price)*1.02, qtyStep)
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

// ValidateStopLoss проверяет отступ SL относительно ATR % волатильности
func ValidateStopLoss(side string, entryPrice, slPrice, maxRiskPct, atr1hPct float64) bool {
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

	// Отступ SL обязан перекрывать рыночный шум 1h ATR минимум на 1.2x
	minRequiredDist := math.Max(1.2, atr1hPct*1.2)

	return distPct >= minRequiredDist && distPct <= maxRiskPct
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

	effectiveMinProfit := math.Max(minProfitPct, 1.5)
	return distPct >= effectiveMinProfit && distPct <= 20.0
}
