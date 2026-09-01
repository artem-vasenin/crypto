// internal/execution/risk.go
package execution

import (
	"fmt"
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

	// Если ноционал ниже требований биржи (например $10), увеличиваем его до $10.5
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

// RoundToStep округляет ценовое значение строго кратно step без плавающей погрешности
func RoundToStep(val, step float64) float64 {
	if step <= 0 || val <= 0 {
		return val
	}
	precision := GetPrecision(step)
	formatStr := "%." + strconv.Itoa(precision) + "f"
	steps := math.Floor(val/step + 0.000000001)
	res := steps * step

	parsed, err := strconv.ParseFloat(fmt.Sprintf(formatStr, res), 64)
	if err != nil {
		return res
	}
	return parsed
}

// GetPrecision определяет количество знаков после запятой у параметра step
func GetPrecision(step float64) int {
	str := strconv.FormatFloat(step, 'f', -1, 64)
	parts := strings.Split(str, ".")
	if len(parts) < 2 {
		return 0
	}
	return len(parts[1])
}

// FormatStep форматирует значение с точностью, заданной step
func FormatStep(val, step float64) string {
	precision := GetPrecision(step)
	return strconv.FormatFloat(val, 'f', precision, 64)
}

// ValidateStopLoss проверяет физическую корректность и границы SL
func ValidateStopLoss(side string, entryPrice, slPrice, minDistancePct float64) bool {
	if slPrice <= 0 || entryPrice <= 0 {
		return false
	}

	distPct := 0.0
	if side == "Buy" {
		if slPrice >= entryPrice {
			return false
		}
		distPct = (entryPrice - slPrice) / entryPrice * 100
	} else if side == "Sell" {
		if slPrice <= entryPrice {
			return false
		}
		distPct = (slPrice - entryPrice) / entryPrice * 100
	}

	return distPct >= minDistancePct && distPct <= 10.0
}

// ValidateTakeProfit проверяет физическую корректность и границы TP
func ValidateTakeProfit(side string, entryPrice, tpPrice, minDistancePct float64) bool {
	if tpPrice <= 0 || entryPrice <= 0 {
		return false
	}

	distPct := 0.0
	if side == "Buy" {
		if tpPrice <= entryPrice {
			return false
		}
		distPct = (tpPrice - entryPrice) / entryPrice * 100
	} else if side == "Sell" {
		if tpPrice >= entryPrice {
			return false
		}
		distPct = (entryPrice - tpPrice) / entryPrice * 100
	}

	return distPct >= minDistancePct && distPct <= 20.0
}
