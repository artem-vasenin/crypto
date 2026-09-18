package indicator

import "math"

// SMA возвращает простое скользящее среднее последнего окна.
func SMA(v []float64, n int) float64 {
	if n <= 0 || len(v) < n {
		return 0
	}
	s := 0.0
	for _, x := range v[len(v)-n:] {
		s += x
	}
	return s / float64(n)
}

// ATR рассчитывает Wilder-like среднее истинного диапазона.
func ATR(h, l, c []float64, n int) float64 {
	if len(c) < n+1 || len(h) != len(c) || len(l) != len(c) {
		return 0
	}
	trs := make([]float64, 0, n)
	for i := len(c) - n; i < len(c); i++ {
		pc := c[i-1]
		tr := math.Max(h[i]-l[i], math.Max(math.Abs(h[i]-pc), math.Abs(l[i]-pc)))
		trs = append(trs, tr)
	}
	return SMA(trs, n)
}

// EfficiencyRatio измеряет направленность движения: 0 — шум, 1 — почти прямой ход.
func EfficiencyRatio(c []float64, n int) float64 {
	if len(c) < n+1 {
		return 0
	}
	start := len(c) - n - 1
	change := math.Abs(c[len(c)-1] - c[start])
	noise := 0.0
	for i := start + 1; i < len(c); i++ {
		noise += math.Abs(c[i] - c[i-1])
	}
	if noise == 0 {
		return 0
	}
	return change / noise
}

// DMIADX возвращает +DI, -DI и ADX по последнему окну.
func DMIADX(h, l, c []float64, n int) (float64, float64, float64) {
	if len(c) < n+1 {
		return 0, 0, 0
	}
	sp, sm, tr := 0.0, 0.0, 0.0
	dxs := []float64{}
	start := len(c) - n
	for i := start; i < len(c); i++ {
		up := h[i] - h[i-1]
		dn := l[i-1] - l[i]
		plus, minus := 0.0, 0.0
		if up > dn && up > 0 {
			plus = up
		}
		if dn > up && dn > 0 {
			minus = dn
		}
		t := math.Max(h[i]-l[i], math.Max(math.Abs(h[i]-c[i-1]), math.Abs(l[i]-c[i-1])))
		sp += plus
		sm += minus
		tr += t
		if tr > 0 {
			p := 100 * sp / tr
			m := 100 * sm / tr
			if p+m > 0 {
				dxs = append(dxs, 100*math.Abs(p-m)/(p+m))
			}
		}
	}
	if tr == 0 {
		return 0, 0, 0
	}
	return 100 * sp / tr, 100 * sm / tr, SMA(dxs, min(n, len(dxs)))
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
