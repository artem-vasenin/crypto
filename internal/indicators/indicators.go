// internal/indicators/indicators.go
package indicators

import (
	"math"
	"universal-bybit-screener/models"
)

// RSI рассчитывает модифицированный сглаженный Wilder RSI.
// При недостаточной длине выборки возвращает 0.
func RSI(candles []models.Candle, period int) float64 {
	if len(candles) <= period || period <= 0 {
		return 0
	}

	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			gainSum += change
		} else {
			lossSum -= change
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	for i := period + 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		var gain, loss float64
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}

		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// ATR рассчитывает абсолютный Average True Range.
func ATR(candles []models.Candle, period int) float64 {
	if len(candles) <= period || period <= 0 {
		return 0
	}

	var trSum float64
	for i := 1; i <= period; i++ {
		trSum += trueRange(candles[i], candles[i-1].Close)
	}

	atr := trSum / float64(period)
	for i := period + 1; i < len(candles); i++ {
		tr := trueRange(candles[i], candles[i-1].Close)
		atr = (atr*float64(period-1) + tr) / float64(period)
	}

	return atr
}

// trueRange вычисляет истинный диапазон свечи
func trueRange(c models.Candle, prevClose float64) float64 {
	highLow := c.High - c.Low
	highPrevClose := math.Abs(c.High - prevClose)
	lowPrevClose := math.Abs(c.Low - prevClose)

	return math.Max(highLow, math.Max(highPrevClose, lowPrevClose))
}

// VolumeRatio сравнивает объем последней свечи со средним объемом предыдущего окна
func VolumeRatio(candles []models.Candle, period int) float64 {
	n := len(candles)
	if n < period+1 || period <= 0 {
		return 0
	}

	var sum float64
	start := n - period - 1
	for i := start; i < n-1; i++ {
		sum += candles[i].Volume
	}

	avg := sum / float64(period)
	if avg == 0 {
		return 0
	}

	return candles[n-1].Volume / avg
}

// VolumeTrend вычисляет отношение короткого скользящего среднего объема к длинному
func VolumeTrend(candles []models.Candle, shortPeriod, longPeriod int) float64 {
	n := len(candles)
	if shortPeriod <= 0 || longPeriod <= 0 || n < shortPeriod+longPeriod {
		return 0
	}

	shortStart := n - shortPeriod
	longStart := shortStart - longPeriod

	var shortSum, longSum float64
	for i := shortStart; i < n; i++ {
		shortSum += candles[i].Volume
	}
	for i := longStart; i < shortStart; i++ {
		longSum += candles[i].Volume
	}

	shortAvg := shortSum / float64(shortPeriod)
	longAvg := longSum / float64(longPeriod)

	if longAvg == 0 {
		return 0
	}

	return shortAvg / longAvg
}
