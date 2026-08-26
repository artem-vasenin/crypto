package indicators

import (
	"math"
	"sort"
	"time"

	"bybit-screener/internal/bybit"
)

func RSI(klines []bybit.Kline, period int) float64 {
	if len(klines) <= period {
		return 50
	}
	gain, loss := 0.0, 0.0
	for i := 1; i <= period; i++ {
		d := klines[i].Close - klines[i-1].Close
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	avgGain, avgLoss := gain/float64(period), loss/float64(period)
	for i := period + 1; i < len(klines); i++ {
		d := klines[i].Close - klines[i-1].Close
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

func RSIAtEnd(klines []bybit.Kline, period int) float64 { return RSI(klines, period) }

func EMA(klines []bybit.Kline, period int) float64 {
	if len(klines) == 0 {
		return 0
	}
	if len(klines) < period {
		period = len(klines)
	}
	value := 0.0
	for i := 0; i < period; i++ {
		value += klines[i].Close
	}
	value /= float64(period)
	alpha := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		value = (klines[i].Close-value)*alpha + value
	}
	return value
}

func ATR(klines []bybit.Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}
	tr := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		a := klines[i].High - klines[i].Low
		b := math.Abs(klines[i].High - klines[i-1].Close)
		c := math.Abs(klines[i].Low - klines[i-1].Close)
		tr = append(tr, math.Max(a, math.Max(b, c)))
	}
	if len(tr) < period {
		return 0
	}
	value := 0.0
	for i := 0; i < period; i++ {
		value += tr[i]
	}
	value /= float64(period)
	for i := period; i < len(tr); i++ {
		value = (value*float64(period-1) + tr[i]) / float64(period)
	}
	return value
}

func ChangePct(klines []bybit.Kline, bars int) float64 {
	if len(klines) <= bars {
		return 0
	}
	old := klines[len(klines)-1-bars].Close
	now := klines[len(klines)-1].Close
	if old == 0 {
		return 0
	}
	return (now - old) / old * 100
}

func VolumeRatio(klines []bybit.Kline, baseline int) float64 {
	if len(klines) < baseline+1 {
		return 0
	}
	current := klines[len(klines)-1].Volume
	sum := 0.0
	start := len(klines) - 1 - baseline
	for i := start; i < len(klines)-1; i++ {
		sum += klines[i].Volume
	}
	avg := sum / float64(baseline)
	if avg == 0 {
		return 0
	}
	return current / avg
}

type Pivot struct {
	Time  time.Time `json:"time"`
	Price float64   `json:"price"`
}

type Structure struct {
	PivotHighs   []Pivot `json:"pivot_highs"`
	PivotLows    []Pivot `json:"pivot_lows"`
	HighState    string  `json:"high_state"`
	LowState     string  `json:"low_state"`
	PreviousHigh float64 `json:"previous_high"`
	CurrentHigh  float64 `json:"current_high"`
	PreviousLow  float64 `json:"previous_low"`
	CurrentLow   float64 `json:"current_low"`
}

func FindPivots(klines []bybit.Kline, window int) Structure {
	if window < 1 {
		window = 2
	}
	highs := make([]Pivot, 0)
	lows := make([]Pivot, 0)
	for i := window; i < len(klines)-window; i++ {
		highOK, lowOK := true, true
		for j := i - window; j <= i+window; j++ {
			if j == i {
				continue
			}
			if klines[j].High >= klines[i].High {
				highOK = false
			}
			if klines[j].Low <= klines[i].Low {
				lowOK = false
			}
		}
		ts := time.UnixMilli(klines[i].StartTime).UTC()
		if highOK {
			highs = append(highs, Pivot{Time: ts, Price: klines[i].High})
		}
		if lowOK {
			lows = append(lows, Pivot{Time: ts, Price: klines[i].Low})
		}
	}
	if len(highs) > 5 {
		highs = highs[len(highs)-5:]
	}
	if len(lows) > 5 {
		lows = lows[len(lows)-5:]
	}
	s := Structure{PivotHighs: highs, PivotLows: lows}
	if len(highs) >= 2 {
		s.PreviousHigh = highs[len(highs)-2].Price
		s.CurrentHigh = highs[len(highs)-1].Price
		if s.CurrentHigh > s.PreviousHigh {
			s.HighState = "HH"
		} else {
			s.HighState = "LH"
		}
	}
	if len(lows) >= 2 {
		s.PreviousLow = lows[len(lows)-2].Price
		s.CurrentLow = lows[len(lows)-1].Price
		if s.CurrentLow > s.PreviousLow {
			s.LowState = "HL"
		} else {
			s.LowState = "LL"
		}
	}
	return s
}

func Range(klines []bybit.Kline, bars int) (low, high, positionPct float64) {
	if len(klines) == 0 {
		return
	}
	if bars > len(klines) {
		bars = len(klines)
	}
	start := len(klines) - bars
	low, high = klines[start].Low, klines[start].High
	for i := start + 1; i < len(klines); i++ {
		low = math.Min(low, klines[i].Low)
		high = math.Max(high, klines[i].High)
	}
	if high > low {
		positionPct = (klines[len(klines)-1].Close - low) / (high - low) * 100
	}
	return
}

func Pearson(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 2 {
		return 0
	}
	ma, mb := 0.0, 0.0
	for i := 0; i < n; i++ {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(n)
	mb /= float64(n)
	var num, da, db float64
	for i := 0; i < n; i++ {
		x := a[i] - ma
		y := b[i] - mb
		num += x * y
		da += x * x
		db += y * y
	}
	if da == 0 || db == 0 {
		return 0
	}
	return num / math.Sqrt(da*db)
}

func Returns(klines []bybit.Kline) []float64 {
	if len(klines) < 2 {
		return nil
	}
	out := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		if klines[i-1].Close == 0 {
			continue
		}
		out = append(out, (klines[i].Close-klines[i-1].Close)/klines[i-1].Close)
	}
	return out
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}
func Min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := values[0]
	for _, x := range values[1:] {
		if x < v {
			v = x
		}
	}
	return v
}
func Max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := values[0]
	for _, x := range values[1:] {
		if x > v {
			v = x
		}
	}
	return v
}
func Std(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := Mean(values)
	s := 0.0
	for _, v := range values {
		s += (v - m) * (v - m)
	}
	return math.Sqrt(s / float64(len(values)-1))
}

func NearestAbove(price float64, levels []float64) float64 {
	best := 0.0
	for _, x := range levels {
		if x > price && (best == 0 || x < best) {
			best = x
		}
	}
	return best
}
func NearestBelow(price float64, levels []float64) float64 {
	best := 0.0
	for _, x := range levels {
		if x < price && (best == 0 || x > best) {
			best = x
		}
	}
	return best
}

func SortFloats(values []float64) []float64 {
	out := append([]float64(nil), values...)
	sort.Float64s(out)
	return out
}
