package indicators

import "math"

type Candle struct{ Open, High, Low, Close, Volume float64 }

func EMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 {
		return out
	}
	if period < 1 {
		period = 1
	}
	k := 2.0 / float64(period+1)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = values[i]*k + out[i-1]*(1-k)
	}
	return out
}

// RSI использует классическое сглаживание Wilder's RMA
func RSI(values []float64, period int) float64 {
	if len(values) < period+1 || period < 1 {
		return 50.0
	}

	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		change := values[i] - values[i-1]
		if change > 0 {
			gainSum += change
		} else {
			lossSum -= change
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	for i := period + 1; i < len(values); i++ {
		change := values[i] - values[i-1]
		gain := 0.0
		loss := 0.0
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}

	if avgLoss == 0 {
		return 100.0
	}
	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}

// ATR по методу Уилдера (Wilder's Smoothing)
func ATR(cs []Candle, period int) float64 {
	if len(cs) < 2 || period < 1 {
		return 0.0
	}

	trs := make([]float64, len(cs)-1)
	for i := 1; i < len(cs); i++ {
		a := cs[i].High - cs[i].Low
		b := math.Abs(cs[i].High - cs[i-1].Close)
		c := math.Abs(cs[i].Low - cs[i-1].Close)
		trs[i-1] = math.Max(a, math.Max(b, c))
	}

	if len(trs) < period {
		period = len(trs)
	}

	var sum float64
	for i := 0; i < period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	for i := period; i < len(trs); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

func PercentChange(values []float64, periods int) float64 {
	if len(values) <= periods || periods < 1 {
		return 0
	}
	base := values[len(values)-1-periods]
	if base == 0 {
		return 0
	}
	return (values[len(values)-1]/base - 1.0) * 100.0
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var s float64
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

func Std(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := Mean(values)
	var s float64
	for _, v := range values {
		s += (v - m) * (v - m)
	}
	return math.Sqrt(s / float64(len(values)))
}

func Correlation(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 3 {
		return 0
	}
	a = a[len(a)-n:]
	b = b[len(b)-n:]
	ma, mb := Mean(a), Mean(b)
	var num, da, db float64
	for i := 0; i < n; i++ {
		x, y := a[i]-ma, b[i]-mb
		num += x * y
		da += x * x
		db += y * y
	}
	if da == 0 || db == 0 {
		return 0
	}
	return num / math.Sqrt(da*db)
}

func VolumeRatio(values []float64, short, long int) float64 {
	if len(values) < long || long == 0 || short == 0 {
		return 0
	}
	a := Mean(values[len(values)-short:])
	b := Mean(values[len(values)-long:])
	if b == 0 {
		return 0
	}
	return a / b
}
