package bybit

import (
	"context"
	"net/url"
	"sort"
	"universal-bybit-screener/models"
)

// OpenInterest возвращает OI в хронологическом порядке, чтобы изменение OI
// считалось как последнее значение относительно самого старого, а не наоборот.
func (c *Client) OpenInterest(ctx context.Context, symbol, interval string, limit int) ([]models.OpenInterestPoint, error) {
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "intervalTime": {interval}, "limit": {fmtInt(limit)}}
	var r struct {
		List []struct {
			OpenInterest string `json:"openInterest"`
			Timestamp    string `json:"timestamp"`
		} `json:"list"`
	}
	if err := c.doGET(ctx, "/v5/market/open-interest", q, &r); err != nil {
		return nil, err
	}
	out := make([]models.OpenInterestPoint, 0, len(r.List))
	for _, x := range r.List {
		out = append(out, models.OpenInterestPoint{Time: ts(x.Timestamp), OpenInterest: f(x.OpenInterest)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}
