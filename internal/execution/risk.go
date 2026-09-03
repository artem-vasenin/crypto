// internal/execution/risk.go
package execution

import (
	"math"
	"strconv"
	"strings"
)

// CalculatePositionQty рассчитывает точный объем позиции с гарантийным покрытием MinNotional Bybit
func CalculatePositionQty(marginUSD float64, leverage int, price, qtyStep, minQty, minNotional float64) float64 {
	if price <= 0 || leverage <= 0 || marginUSD <= 0 || qtyStep <= 0 {
		return 0
	}

	notionalUSD := marginUSD * float64(leverage)
	targetMinNotional := math.Max(minNotional, 10.5)
	if notionalUSD < targetMinNotional {
		notionalUSD = targetMinNotional
	}

	rawQty := notionalUSD / price
	if rawQty < minQty {
		rawQty = minQty
	}

	steps := math.Floor(rawQty / qtyStep)
	qty := steps * qtyStep

	precision := GetPrecision(qtyStep)
	factor := math.Pow(10, float64(precision))
	qty = math.Floor(qty*factor+0.5) / factor

	if qty < minQty {
		return 0
	}

	return qty
}

// RoundToStep атомарно округляет значение кратно step без накладных расходов на ParseFloat
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

// ValidateStopLoss проверяет физическую корректность и границы SL.
// maxRiskPct (например 2.5%) — это МАКСИМАЛЬНО допустимый риск, а не минимальный!
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

	// Минимальный отступ 0.8% закладывается от рыночного шума и спреда.
	// Риск позиции НЕ ДОЛЖЕН ПРЕВЫШАТЬ maxRiskPct.
	return distPct >= 0.8 && distPct <= maxRiskPct
}

// ValidateTakeProfit проверяет физическую корректность и границы TP
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

	return distPct >= minProfitPct && distPct <= 20.0
}
