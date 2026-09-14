package execution

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"universal-bybit-screener/models"
)

// CalculateDynamicStopLoss places the stop beyond the relevant pivot and at least
// atrMultiplier ATRs away from entry. A missing or invalid pivot is never treated
// as a real price level.
func CalculateDynamicStopLoss(side string, entryPrice, pivotLevel, atr1h float64, atrMultiplier float64, tickStep float64) float64 {
	if entryPrice <= 0 || atr1h <= 0 {
		return 0
	}
	if atrMultiplier < 1.5 {
		atrMultiplier = 1.5
	}

	minDistance := atr1h * atrMultiplier
	distance := minDistance

	if strings.EqualFold(side, "Buy") && pivotLevel > 0 && pivotLevel < entryPrice {
		distance = math.Max(entryPrice-pivotLevel, minDistance)
	}
	if strings.EqualFold(side, "Sell") && pivotLevel > entryPrice {
		distance = math.Max(pivotLevel-entryPrice, minDistance)
	}

	switch {
	case strings.EqualFold(side, "Buy"):
		return roundPriceDown(entryPrice-distance, tickStep)
	case strings.EqualFold(side, "Sell"):
		return roundPriceUp(entryPrice+distance, tickStep)
	default:
		return 0
	}
}

// CalculateTrailingStop computes a monotonic trailing SL candidate from a fresh
// market price. Trailing activates only after the position has moved 1R in its
// favor and ignores SL improvements smaller than minMovePct to avoid chasing
// microstructure noise.
func CalculateTrailingStop(side string, entryPrice, currentSL, price, previousExtreme, trailingPct, minMovePct, tickStep float64) (newSL, extreme float64, active bool, reason string) {
	if entryPrice <= 0 || currentSL <= 0 || price <= 0 || trailingPct <= 0 {
		return 0, previousExtreme, false, "invalid_parameters"
	}

	initialRisk := math.Abs(entryPrice - currentSL)
	if initialRisk <= 0 {
		return 0, previousExtreme, false, "invalid_initial_risk"
	}

	switch {
	case strings.EqualFold(side, "Buy"):
		if price < entryPrice+initialRisk {
			return 0, previousExtreme, false, "activation_not_reached"
		}
		extreme = math.Max(previousExtreme, price)
		newSL = roundPriceDown(extreme*(1-trailingPct/100), tickStep)
	case strings.EqualFold(side, "Sell"):
		if price > entryPrice-initialRisk {
			return 0, previousExtreme, false, "activation_not_reached"
		}
		if previousExtreme <= 0 {
			extreme = price
		} else {
			extreme = math.Min(previousExtreme, price)
		}
		newSL = roundPriceUp(extreme*(1+trailingPct/100), tickStep)
	default:
		return 0, previousExtreme, false, "invalid_side"
	}

	if newSL <= 0 {
		return 0, extreme, true, "invalid_candidate_sl"
	}
	if (strings.EqualFold(side, "Buy") && newSL <= currentSL) ||
		(strings.EqualFold(side, "Sell") && newSL >= currentSL) {
		return newSL, extreme, true, "sl_not_improved"
	}

	if minMovePct > 0 {
		movePct := math.Abs(newSL-currentSL) / currentSL * 100
		if movePct < minMovePct {
			return newSL, extreme, true, "sl_move_too_small"
		}
	}

	return newSL, extreme, true, "new_extreme"
}

func CalculateDynamicTakeProfit(side string, entryPrice, slPrice, minRR float64, tickStep float64) float64 {
	if entryPrice <= 0 || slPrice <= 0 {
		return 0
	}
	if minRR < 2 {
		minRR = 2
	}

	distance := math.Abs(entryPrice - slPrice)
	switch {
	case strings.EqualFold(side, "Buy"):
		return roundPriceUp(entryPrice+distance*minRR, tickStep)
	case strings.EqualFold(side, "Sell"):
		return roundPriceDown(entryPrice-distance*minRR, tickStep)
	default:
		return 0
	}
}

func CalculateDynamicLeverage(c models.Candidate, targetStrategy string, maxLeverage int) int {
	if maxLeverage <= 1 {
		return 1
	}

	res, ok := c.Strategies[targetStrategy]
	if !ok || !res.Decision.Eligible {
		return 1
	}

	leverage := maxLeverage
	if c.Indicators.ATR1hPct > 3 {
		leverage = maxLeverage / 2
	}
	if leverage < 1 {
		leverage = 1
	}
	return leverage
}

// CalculatePositionQty never silently enlarges a trade just to satisfy exchange
// minimums. If the configured margin cannot satisfy the exchange minimum,
// the caller must reject the trade.
func CalculatePositionQty(marginUSD float64, leverage int, price, qtyStep, minQty, minNotional, maxOrderQty float64) (float64, error) {
	if price <= 0 || leverage <= 0 || marginUSD <= 0 || qtyStep <= 0 {
		return 0, fmt.Errorf("invalid position sizing parameters")
	}

	targetNotional := marginUSD * float64(leverage)
	if minNotional > 0 && targetNotional < minNotional {
		return 0, fmt.Errorf("configured margin produces notional %.4f below exchange minimum %.4f", targetNotional, minNotional)
	}

	qty := floorToStep(targetNotional/price, qtyStep)
	if qty <= 0 || (minQty > 0 && qty < minQty) {
		return 0, fmt.Errorf("calculated quantity %.12f is below exchange minimum %.12f", qty, minQty)
	}
	if maxOrderQty > 0 && qty > maxOrderQty {
		qty = floorToStep(maxOrderQty, qtyStep)
	}
	if qty <= 0 || qty*price < minNotional {
		return 0, fmt.Errorf("quantity after exchange rounding violates minimum notional")
	}

	return qty, nil
}

func RoundToStep(val, step float64) float64 {
	if step <= 0 || val <= 0 {
		return val
	}
	return math.Round(val/step) * step
}

func floorToStep(val, step float64) float64 {
	if step <= 0 || val <= 0 {
		return val
	}
	return math.Floor((val/step)+1e-12) * step
}

func roundPriceDown(val, step float64) float64 {
	if step <= 0 || val <= 0 {
		return val
	}
	return floorToStep(val, step)
}

func roundPriceUp(val, step float64) float64 {
	if step <= 0 || val <= 0 {
		return val
	}
	return math.Ceil((val/step)-1e-12) * step
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
	if step <= 0 {
		return strconv.FormatFloat(val, 'f', -1, 64)
	}
	return strconv.FormatFloat(RoundToStep(val, step), 'f', GetPrecision(step), 64)
}

func ValidateStopLoss(side string, entryPrice, slPrice, maxRiskPct, atr1hPct float64) bool {
	if slPrice <= 0 || entryPrice <= 0 || maxRiskPct <= 0 {
		return false
	}

	var distPct float64
	switch {
	case strings.EqualFold(side, "Buy") && slPrice < entryPrice:
		distPct = (entryPrice - slPrice) / entryPrice * 100
	case strings.EqualFold(side, "Sell") && slPrice > entryPrice:
		distPct = (slPrice - entryPrice) / entryPrice * 100
	default:
		return false
	}

	minRequired := math.Max(0.8, atr1hPct)
	return distPct >= minRequired && distPct <= maxRiskPct
}

func ValidateTakeProfit(side string, entryPrice, tpPrice, minProfitPct float64) bool {
	if tpPrice <= 0 || entryPrice <= 0 || minProfitPct <= 0 {
		return false
	}

	var distPct float64
	switch {
	case strings.EqualFold(side, "Buy") && tpPrice > entryPrice:
		distPct = (tpPrice - entryPrice) / entryPrice * 100
	case strings.EqualFold(side, "Sell") && tpPrice < entryPrice:
		distPct = (entryPrice - tpPrice) / entryPrice * 100
	default:
		return false
	}

	return distPct >= minProfitPct
}

func EstimatedRoundTripCostPct(makerRate, takerRate, extraCostPct float64) float64 {
	// Entry is intended to be maker. TP/SL are full-position market orders,
	// therefore the conservative estimate uses one maker + one taker fee.
	return (makerRate+takerRate)*100*1.25 + extraCostPct
}
