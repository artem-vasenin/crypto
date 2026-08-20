package indicators

import (
	"cs/model"
	"math"
)

// RSI рассчитывает Relative Strength Index.
//
// Используем классический RSI с периодом 14.
//
// Важный момент:
// RSI > 70 НЕ означает автоматически Short.
//
// Для нашего скринера RSI — только один из факторов.
func RSI(candles []model.Candle, period int) float64 {

	if len(candles) <= period {
		return 0
	}

	var gains float64
	var losses float64

	// Первое значение RSI рассчитываем
	// через средний gain/loss.
	for i := 1; i <= period; i++ {

		change := candles[i].Close - candles[i-1].Close

		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	// Wilder smoothing.
	for i := period + 1; i < len(candles); i++ {

		change := candles[i].Close - candles[i-1].Close

		gain := 0.0
		loss := 0.0

		if change > 0 {
			gain = change
		} else {
			loss = -change
		}

		avgGain =
			(avgGain*float64(period-1) + gain) /
				float64(period)

		avgLoss =
			(avgLoss*float64(period-1) + loss) /
				float64(period)
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss

	return 100 - (100 / (1 + rs))
}

// ATR рассчитывает Average True Range.
//
// ATR показывает абсолютную волатильность.
//
// Чем больше ATR относительно цены,
// тем сильнее двигается монета.
func ATR(candles []model.Candle, period int) float64 {

	if len(candles) <= period {
		return 0
	}

	trueRanges := make(
		[]float64,
		0,
		len(candles)-1,
	)

	for i := 1; i < len(candles); i++ {

		current := candles[i]
		previous := candles[i-1]

		// True Range — максимум из:
		//
		// High - Low
		// |High - Previous Close|
		// |Low - Previous Close|
		tr := math.Max(
			current.High-current.Low,
			math.Max(
				math.Abs(current.High-previous.Close),
				math.Abs(current.Low-previous.Close),
			),
		)

		trueRanges = append(trueRanges, tr)
	}

	if len(trueRanges) < period {
		return 0
	}

	// Первоначальный ATR.
	var sum float64

	for i := 0; i < period; i++ {
		sum += trueRanges[i]
	}

	atr := sum / float64(period)

	// Wilder smoothing.
	for i := period; i < len(trueRanges); i++ {

		atr =
			(atr*float64(period-1) + trueRanges[i]) /
				float64(period)
	}

	return atr
}

// ATRPercent переводит ATR в проценты от текущей цены.
func ATRPercent(
	candles []model.Candle,
	period int,
) float64 {

	if len(candles) == 0 {
		return 0
	}

	atr := ATR(candles, period)

	close := candles[len(candles)-1].Close

	if close == 0 {
		return 0
	}

	return atr / close * 100
}

// VolumeRatio показывает,
// насколько объём последней свечи отличается
// от среднего объёма предыдущих свечей.
//
// Например:
//
// 1.0 = примерно средний объём
// 2.0 = в два раза выше среднего
// 0.5 = в два раза ниже среднего
func VolumeRatio(
	candles []model.Candle,
	period int,
) float64 {

	if len(candles) <= period {
		return 0
	}

	last := candles[len(candles)-1]

	var sum float64

	start := len(candles) - period - 1
	end := len(candles) - 1

	for i := start; i < end; i++ {
		sum += candles[i].Volume
	}

	average := sum / float64(period)

	if average == 0 {
		return 0
	}

	return last.Volume / average
}

// HighestHigh возвращает максимальный High
// за последние candles.
func HighestHigh(candles []model.Candle) float64 {

	if len(candles) == 0 {
		return 0
	}

	result := candles[0].High

	for i := 1; i < len(candles); i++ {
		if candles[i].High > result {
			result = candles[i].High
		}
	}

	return result
}

// LowestLow возвращает минимальный Low.
func LowestLow(candles []model.Candle) float64 {

	if len(candles) == 0 {
		return 0
	}

	result := candles[0].Low

	for i := 1; i < len(candles); i++ {
		if candles[i].Low < result {
			result = candles[i].Low
		}
	}

	return result
}
