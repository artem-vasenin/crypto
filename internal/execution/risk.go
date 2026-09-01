// internal/execution/risk.go
package execution

import (
	"math"
)

// CalculatePositionQty вычисляет объем контрактов на основе размера маржи в USD и выставляемого плеча
func CalculatePositionQty(marginUSD float64, leverage int, currentPrice, qtyStep float64) float64 {
	if currentPrice <= 0 || leverage <= 0 || marginUSD <= 0 {
		return 0
	}

	notionalUSD := marginUSD * float64(leverage)
	rawQty := notionalUSD / currentPrice

	return RoundToStep(rawQty, qtyStep)
}

// RoundToStep округляет количество вниз с точностью до допустимого шага (qtyStep / tickSize)
func RoundToStep(val, step float64) float64 {
	if step <= 0 {
		return val
	}
	precision := math.Round(1 / step)
	return math.Floor(val*precision) / precision
}

// ValidateStopLoss проверяет минимальный зазор стоп-лосса во избежание мгновенной ликвидации при открытии
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

	return distPct >= minDistancePct
}
