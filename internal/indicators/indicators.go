// Package indicators содержит математические функции без сетевых зависимостей.
package indicators

import "math"

type Candle struct{ Open, High, Low, Close, Volume float64 }

func Mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
func Std(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := Mean(v)
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)))
}

// EMA возвращает всю последовательность экспоненциальной средней.
func EMA(v []float64, p int) []float64 {
	o := make([]float64, len(v))
	if len(v) == 0 {
		return o
	}
	if p < 1 {
		p = 1
	}
	k := 2.0 / float64(p+1)
	o[0] = v[0]
	for i := 1; i < len(v); i++ {
		o[i] = v[i]*k + o[i-1]*(1-k)
	}
	return o
}

// RSI вычисляет RSI Уайлдера по всей доступной истории.
func RSI(v []float64, p int) float64 {
	if p < 1 || len(v) < p+1 {
		return 50
	}
	g, l := 0.0, 0.0
	for i := 1; i <= p; i++ {
		d := v[i] - v[i-1]
		if d > 0 {
			g += d
		} else {
			l -= d
		}
	}
	g /= float64(p)
	l /= float64(p)
	for i := p + 1; i < len(v); i++ {
		d := v[i] - v[i-1]
		gg, ll := 0.0, 0.0
		if d > 0 {
			gg = d
		} else {
			ll = -d
		}
		g = (g*float64(p-1) + gg) / float64(p)
		l = (l*float64(p-1) + ll) / float64(p)
	}
	if l == 0 {
		return 100
	}
	rs := g / l
	return 100 - 100/(1+rs)
}

// ATR вычисляет Average True Range Уайлдера.
func ATR(c []Candle, p int) float64 {
	if len(c) < 2 {
		return 0
	}
	tr := make([]float64, len(c)-1)
	for i := 1; i < len(c); i++ {
		a := c[i].High - c[i].Low
		b := math.Abs(c[i].High - c[i-1].Close)
		d := math.Abs(c[i].Low - c[i-1].Close)
		tr[i-1] = math.Max(a, math.Max(b, d))
	}
	if p > len(tr) {
		p = len(tr)
	}
	if p < 1 {
		return 0
	}
	a := Mean(tr[:p])
	for i := p; i < len(tr); i++ {
		a = (a*float64(p-1) + tr[i]) / float64(p)
	}
	return a
}

// PercentChange возвращает изменение между последним значением и N периодов назад.
func PercentChange(v []float64, n int) float64 {
	if n < 1 || len(v) <= n {
		return 0
	}
	b := v[len(v)-1-n]
	if b == 0 {
		return 0
	}
	return (v[len(v)-1]/b - 1) * 100
}

// Correlation — коэффициент Пирсона по двум хвостам одинаковой длины.
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
	num, da, db := 0.0, 0.0, 0.0
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

// EfficiencyRatio показывает направленность движения: около 1 — тренд, около 0 — шум/mean reversion.
func EfficiencyRatio(v []float64, n int) float64 {
	if len(v) < 2 {
		return 0
	}
	if n >= len(v) {
		n = len(v) - 1
	}
	if n < 1 {
		return 0
	}
	s := len(v) - 1 - n
	net := math.Abs(v[len(v)-1] - v[s])
	path := 0.0
	for i := s + 1; i < len(v); i++ {
		path += math.Abs(v[i] - v[i-1])
	}
	if path == 0 {
		return 0
	}
	return net / path
}

// RealizedVolPct — стандартное отклонение простых доходностей в процентах.
func RealizedVolPct(v []float64, n int) float64 {
	if len(v) < 3 {
		return 0
	}
	if n >= len(v) {
		n = len(v) - 1
	}
	r := make([]float64, 0, n)
	for i := len(v) - n; i < len(v); i++ {
		if i > 0 && v[i-1] != 0 {
			r = append(r, (v[i]/v[i-1]-1)*100)
		}
	}
	return Std(r)
}

// BollingerWidthPct возвращает ширину полос ±2σ относительно средней.
func BollingerWidthPct(v []float64, n int) float64 {
	if len(v) == 0 {
		return 0
	}
	if n > len(v) {
		n = len(v)
	}
	x := v[len(v)-n:]
	m := Mean(x)
	if m == 0 {
		return 0
	}
	return 4 * Std(x) / m * 100
}

// ADX реализует классический ADX Уайлдера и нужен как независимая оценка силы тренда.
func ADX(c []Candle, p int) float64 {
	if len(c) < p*2+1 || p < 2 {
		return 0
	}
	trs, pdm, mdm := make([]float64, len(c)-1), make([]float64, len(c)-1), make([]float64, len(c)-1)
	for i := 1; i < len(c); i++ {
		up := c[i].High - c[i-1].High
		dn := c[i-1].Low - c[i].Low
		if up > dn && up > 0 {
			pdm[i-1] = up
		}
		if dn > up && dn > 0 {
			mdm[i-1] = dn
		}
		trs[i-1] = math.Max(c[i].High-c[i].Low, math.Max(math.Abs(c[i].High-c[i-1].Close), math.Abs(c[i].Low-c[i-1].Close)))
	}
	tr, pp, mm := 0.0, 0.0, 0.0
	for i := 0; i < p; i++ {
		tr += trs[i]
		pp += pdm[i]
		mm += mdm[i]
	}
	dx := make([]float64, 0, len(trs)-p+1)
	for i := p; i <= len(trs); i++ {
		if i > p {
			tr = tr - tr/float64(p) + trs[i-1]
			pp = pp - pp/float64(p) + pdm[i-1]
			mm = mm - mm/float64(p) + mdm[i-1]
		}
		if tr == 0 {
			continue
		}
		pdi := 100 * pp / tr
		mdi := 100 * mm / tr
		den := pdi + mdi
		if den > 0 {
			dx = append(dx, 100*math.Abs(pdi-mdi)/den)
		}
	}
	if len(dx) < p {
		return Mean(dx)
	}
	adx := Mean(dx[:p])
	for i := p; i < len(dx); i++ {
		adx = (adx*float64(p-1) + dx[i]) / float64(p)
	}
	return adx
}
