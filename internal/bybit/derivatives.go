// internal/bybit/derivatives.go
package bybit

import (
	"context"
	"net/url"
	"sort"
	"universal-bybit-screener/models"
)

// OpenInterest запрашивает историческую последовательность Open Interest
func (c *Client) OpenInterest(ctx context.Context, symbol, interval string, limit int) ([]models.OpenInterestPoint, error) {
	q := url.Values{
		"category":     {"linear"},
		"symbol":       {symbol},
		"intervalTime": {interval},
		"limit":        {fmtInt(limit)},
	}

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
		out = append(out, models.OpenInterestPoint{
			Time:         ts(x.Timestamp),
			OpenInterest: f(x.OpenInterest),
		})
	}

	// Сортировка по возрастанию времени
	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})

	return out, nil
}

// Funding запрашивает историю ставок финансирования (Funding Rate)
func (c *Client) Funding(ctx context.Context, symbol string, limit int) ([]models.FundingPoint, error) {
	q := url.Values{
		"category": {"linear"},
		"symbol":   {symbol},
		"limit":    {fmtInt(limit)},
	}

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
		out = append(out, models.FundingPoint{
			Time: ts(x.FundingRateTimestamp),
			Rate: f(x.FundingRate),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})

	return out, nil
}
