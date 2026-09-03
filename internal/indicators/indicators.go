// Базовые технические индикаторы. Все расчёты сделаны без сторонних библиотек,
// чтобы проект можно было собрать обычным go build.
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

func RSI(values []float64, period int) float64 {
	if len(values) < period+1 {
		return 50
	}
	var gain, loss float64
	for i := len(values) - period; i < len(values); i++ {
		d := values[i] - values[i-1]
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	if loss == 0 {
		return 100
	}
	rs := (gain / float64(period)) / (loss / float64(period))
	return 100 - 100/(1+rs)
}

func ATR(cs []Candle, period int) float64 {
	if len(cs) < 2 {
		return 0
	}
	trs := make([]float64, 0, len(cs)-1)
	for i := 1; i < len(cs); i++ {
		a := cs[i].High - cs[i].Low
		b := math.Abs(cs[i].High - cs[i-1].Close)
		c := math.Abs(cs[i].Low - cs[i-1].Close)
		trs = append(trs, math.Max(a, math.Max(b, c)))
	}
	if len(trs) < period {
		period = len(trs)
	}
	var s float64
	for _, v := range trs[len(trs)-period:] {
		s += v
	}
	return s / float64(period)
}

func PercentChange(values []float64, periods int) float64 {
	if len(values) <= periods || values[len(values)-1-periods] == 0 {
		return 0
	}
	return (values[len(values)-1]/values[len(values)-1-periods] - 1) * 100
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
func Returns(values []float64) []float64 {
	if len(values) < 2 {
		return nil
	}
	out := make([]float64, 0, len(values)-1)
	for i := 1; i < len(values); i++ {
		if values[i-1] != 0 {
			out = append(out, values[i]/values[i-1]-1)
		}
	}
	return out
}
func VolumeRatio(values []float64, short, long int) float64 {
	if len(values) < long || long == 0 {
		return 0
	}
	a := Mean(values[len(values)-short:])
	b := Mean(values[len(values)-long:])
	if b == 0 {
		return 0
	}
	return a / b
}
