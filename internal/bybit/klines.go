package bybit

import (
	"context"
	"net/url"
	"sc/models"
	"sort"
)

// Klines разворачивает ответ Bybit в хронологический порядок: старые свечи слева.
func (c *Client) Klines(ctx context.Context, symbol, interval string, limit int) ([]models.Candle, error) {
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "interval": {interval}, "limit": {fmtInt(limit)}}
	var r struct {
		List [][]string `json:"list"`
	}
	if err := c.doGET(ctx, "/v5/market/kline", q, &r); err != nil {
		return nil, err
	}
	out := make([]models.Candle, 0, len(r.List))
	for _, x := range r.List {
		if len(x) < 7 {
			continue
		}
		out = append(out, models.Candle{Time: ts(x[0]), Open: f(x[1]), High: f(x[2]), Low: f(x[3]), Close: f(x[4]), Volume: f(x[5]), Turnover: f(x[6])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}
