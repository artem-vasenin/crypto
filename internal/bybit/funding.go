package bybit

import (
	"context"
	"net/url"
	"sc/models"
	"sort"
)

// Funding также сортируем по времени, чтобы средние и изменения не зависели
// от порядка, в котором API вернул историю.
func (c *Client) Funding(ctx context.Context, symbol string, limit int) ([]models.FundingPoint, error) {
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "limit": {fmtInt(limit)}}
	var r struct {
		List []struct {
			FundingRate          string `json:"fundingRate"`
			FundingRateTimestamp string `json:"fundingRateTimestamp"`
		} `json:"list"`
	}
	if err := c.doGET(ctx, "/v5/market/funding/history", q, &r); err != nil {
		return nil, err
	}
	out := make([]models.FundingPoint, 0, len(r.List))
	for _, x := range r.List {
		out = append(out, models.FundingPoint{Time: ts(x.FundingRateTimestamp), Rate: f(x.FundingRate)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}
