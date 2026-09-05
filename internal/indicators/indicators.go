// internal/indicators/indicators.go
package indicators

import (
	"math"
	"universal-bybit-screener/models"
)

// RSI рассчитывает Relative Strength Index по методу Уайдлера (Wilder's Smoothing)
func RSI(candles []models.Candle, period int) float64 {
	if len(candles) <= period || period <= 0 {
		return 50.0
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
		if avgGain == 0 {
			return 50.0 // Полный флэт
		}
		return 100.0
	}

	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}

// ATR рассчитывает Average True Range с использованием Wilder's Smoothing
func ATR(candles []models.Candle, period int) float64 {
	if len(candles) <= period || period <= 0 {
		return 0.0
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

func trueRange(c models.Candle, prevClose float64) float64 {
	highLow := c.High - c.Low
	highPrevClose := math.Abs(c.High - prevClose)
	lowPrevClose := math.Abs(c.Low - prevClose)

	return math.Max(highLow, math.Max(highPrevClose, lowPrevClose))
}

// VolumeRatio оценивает аномальный приток денег в USDT строго по ЗАКРЫТЫМ свечам
func VolumeRatio(candles []models.Candle, period int) float64 {
	n := len(candles)
	// Требуется как минимум period + 2 свечи, чтобы отбросить текущую незакрытую [n-1]
	if n < period+2 || period <= 0 {
		return 1.0
	}

	// Исключаем последнюю свечу [n-1], так как она еще формируется
	completed := candles[:n-1]
	lastCompleted := completed[len(completed)-1]

	var sum float64
	start := len(completed) - period
	for i := start; i < len(completed)-1; i++ {
		sum += completed[i].Turnover
	}

	avg := sum / float64(period)
	if avg == 0 {
		return 1.0
	}

	return lastCompleted.Turnover / avg
}

func VolumeTrend(candles []models.Candle, shortPeriod, longPeriod int) float64 {
	n := len(candles)
	if shortPeriod <= 0 || longPeriod <= 0 || n < shortPeriod+longPeriod {
		return 0.0
	}

	shortStart := n - shortPeriod
	longStart := shortStart - longPeriod

	var shortSum, longSum float64
	for i := shortStart; i < n; i++ {
		shortSum += candles[i].Turnover
	}
	for i := longStart; i < shortStart; i++ {
		longSum += candles[i].Turnover
	}

	shortAvg := shortSum / float64(shortPeriod)
	longAvg := longSum / float64(longPeriod)

	if longAvg == 0 {
		return 0.0
	}

	return (shortAvg/longAvg - 1.0) * 100.0
}
