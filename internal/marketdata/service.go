package marketdata

import (
	"candidate-screener/internal/bybit"
	"candidate-screener/internal/config"
	"candidate-screener/internal/domain"
	"candidate-screener/internal/features"
	"context"
	"time"
)

// BuildSnapshot собирает минимально достаточные 15m/1h/4h данные и запускает общее feature-ядро.
func BuildSnapshot(ctx context.Context, c *bybit.Client, i domain.Instrument, t domain.Ticker, cfg config.Config) (domain.MarketSnapshot, error) {
	s := domain.MarketSnapshot{Symbol: i.Symbol, GeneratedAt: time.Now().UTC(), Price: t.LastPrice, Turnover24h: t.Turnover24h, FundingRate: t.FundingRate, OpenInterest: t.OpenInterest, TF: map[string]domain.TimeframeFeatures{}, Evidence: []string{}, Conflicts: []string{}}
	for _, x := range []struct {
		name, api string
		limit     int
	}{{"15m", "15", cfg.MinHistoryBars}, {"1h", "60", cfg.MinHistoryBars}, {"4h", "240", 180}} {
		k, e := c.Klines(ctx, i.Symbol, x.api, x.limit)
		if e != nil {
			return s, e
		}
		s.TF[x.name] = features.ExtractTimeframe(x.name, k)
	}
	features.ClassifyMarket(&s, cfg)
	return s, nil
}
