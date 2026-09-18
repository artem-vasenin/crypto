package universe

import (
	"candidate-screener/internal/domain"
	"sort"
)

// Select применяет только дешёвые предварительные фильтры ликвидности и качества инструмента.
func Select(inst []domain.Instrument, t map[string]domain.Ticker, minTurnover float64) []domain.Instrument {
	out := []domain.Instrument{}
	for _, i := range inst {
		x, ok := t[i.Symbol]
		if !ok || x.Turnover24h < minTurnover || x.LastPrice <= 0 {
			continue
		}
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return t[out[a].Symbol].Turnover24h > t[out[b].Symbol].Turnover24h })
	return out
}
