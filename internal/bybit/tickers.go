package bybit

import (
	"context"
	"net/url"
	"sc/models"
)

// Tickers получает snapshot всех linear-контрактов одним запросом.
func (c *Client) Tickers(ctx context.Context) (map[string]models.Ticker, error) {
	q := url.Values{"category": {"linear"}}
	var raw struct {
		List []map[string]string `json:"list"`
	}
	if err := c.doGET(ctx, "/v5/market/tickers", q, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]models.Ticker, len(raw.List))
	for _, x := range raw.List {
		out[x["symbol"]] = models.Ticker{Symbol: x["symbol"], LastPrice: f(x["lastPrice"]), IndexPrice: f(x["indexPrice"]), MarkPrice: f(x["markPrice"]), Price24hPcnt: f(x["price24hPcnt"]) * 100, HighPrice24h: f(x["highPrice24h"]), LowPrice24h: f(x["lowPrice24h"]), Turnover24h: f(x["turnover24h"]), Volume24h: f(x["volume24h"]), Bid1Price: f(x["bid1Price"]), Ask1Price: f(x["ask1Price"]), FundingRate: f(x["fundingRate"]), OpenInterest: f(x["openInterest"]), OpenInterestValue: f(x["openInterestValue"])}
	}
	return out, nil
}
