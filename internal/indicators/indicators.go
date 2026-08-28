package indicators

import "sc/models"

// RSI считает Wilder RSI. Ноль означает, что истории недостаточно.
func RSI(c []models.Candle, period int) float64 {
	if len(c) <= period || period <= 0 {
		return 0
	}
	gain, loss := 0.0, 0.0
	for i := 1; i <= period; i++ {
		d := c[i].Close - c[i-1].Close
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	avgGain, avgLoss := gain/float64(period), loss/float64(period)
	for i := period + 1; i < len(c); i++ {
		d := c[i].Close - c[i-1].Close
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
	}
	if avgLoss == 0 {
		return 100
	}
	return 100 - 100/(1+avgGain/avgLoss)
}

// ATR возвращает абсолютную среднюю величину истинного диапазона.
func ATR(c []models.Candle, period int) float64 {
	if len(c) <= period || period <= 0 {
		return 0
	}
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += tr(c[i], c[i-1].Close)
	}
	atr := sum / float64(period)
	for i := period + 1; i < len(c); i++ {
		atr = (atr*float64(period-1) + tr(c[i], c[i-1].Close)) / float64(period)
	}
	return atr
}
func tr(c models.Candle, prev float64) float64 {
	a := c.High - c.Low
	b := abs(c.High - prev)
	d := abs(c.Low - prev)
	if b > a {
		a = b
	}
	if d > a {
		a = d
	}
	return a
}
func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// VolumeRatio сравнивает последний объём со средним предыдущего окна.
func VolumeRatio(c []models.Candle, period int) float64 {
	if len(c) < period+1 || period <= 0 {
		return 0
	}
	sum := 0.0
	start := len(c) - period - 1
	for i := start; i < len(c)-1; i++ {
		sum += c[i].Volume
	}
	avg := sum / float64(period)
	if avg == 0 {
		return 0
	}
	return c[len(c)-1].Volume / avg
}
