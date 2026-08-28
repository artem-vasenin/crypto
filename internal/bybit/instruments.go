package bybit

import (
	"context"
	"net/url"
	"universal-bybit-screener/models"
)

// Instruments проходит pagination и оставляет только USDT linear perpetual.
func (c *Client) Instruments(ctx context.Context) ([]models.Instrument, error) {
	var out []models.Instrument
	cursor := ""
	for {
		q := url.Values{"category": {"linear"}, "status": {"Trading"}, "limit": {"1000"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var r struct {
			List []struct {
				Symbol, BaseCoin, QuoteCoin, SettleCoin, ContractType, Status string
				LaunchTime                                                    string `json:"launchTime"`
				PriceFilter                                                   struct {
					TickSize string `json:"tickSize"`
				} `json:"priceFilter"`
				LotSizeFilter struct {
					MinOrderQty      string `json:"minOrderQty"`
					QtyStep          string `json:"qtyStep"`
					MinNotionalValue string `json:"minNotionalValue"`
				} `json:"lotSizeFilter"`
				FundingInterval int `json:"fundingInterval"`
			} `json:"list"`
			NextPageCursor string `json:"nextPageCursor"`
		}
		if err := c.doGET(ctx, "/v5/market/instruments-info", q, &r); err != nil {
			return nil, err
		}
		for _, x := range r.List {
			if x.ContractType != "LinearPerpetual" || x.QuoteCoin != "USDT" || x.SettleCoin != "USDT" {
				continue
			}
			out = append(out, models.Instrument{Symbol: x.Symbol, BaseCoin: x.BaseCoin, QuoteCoin: x.QuoteCoin, SettleCoin: x.SettleCoin, ContractType: x.ContractType, Status: x.Status, LaunchTime: ts(x.LaunchTime), TickSize: f(x.PriceFilter.TickSize), MinOrderQty: f(x.LotSizeFilter.MinOrderQty), QtyStep: f(x.LotSizeFilter.QtyStep), MinNotional: f(x.LotSizeFilter.MinNotionalValue), FundingInterval: x.FundingInterval})
		}
		if r.NextPageCursor == "" {
			break
		}
		cursor = r.NextPageCursor
	}
	return out, nil
}
