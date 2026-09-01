// internal/bybit/tickers.go
package bybit

import (
	"context"
	"net/url"
	"universal-bybit-screener/models"
)

// Tickers запрашивает текущий 24h снимок рынка для всех USDT Linear контрактов
func (c *Client) Tickers(ctx context.Context) (map[string]models.Ticker, error) {
	q := url.Values{
		"category": {"linear"},
	}

	var r struct {
		List []struct {
			Symbol            string `json:"symbol"`
			LastPrice         string `json:"lastPrice"`
			MarkPrice         string `json:"markPrice"`
			IndexPrice        string `json:"indexPrice"`
			Price24hPcnt      string `json:"price24hPcnt"`
			HighPrice24h      string `json:"highPrice24h"`
			LowPrice24h       string `json:"lowPrice24h"`
			Turnover24h       string `json:"turnover24h"`
			Volume24h         string `json:"volume24h"`
			Bid1Price         string `json:"bid1Price"`
			Ask1Price         string `json:"ask1Price"`
			FundingRate       string `json:"fundingRate"`
			OpenInterest      string `json:"openInterest"`
			OpenInterestValue string `json:"openInterestValue"`
		} `json:"list"`
	}

	if err := c.doGET(ctx, "/v5/market/tickers", q, &r); err != nil {
		return nil, err
	}

	out := make(map[string]models.Ticker, len(r.List))
	for _, x := range r.List {
		out[x.Symbol] = models.Ticker{
			Symbol:            x.Symbol,
			LastPrice:         f(x.LastPrice),
			MarkPrice:         f(x.MarkPrice),
			IndexPrice:        f(x.IndexPrice),
			Price24hPcnt:      f(x.Price24hPcnt) * 100, // Конвертация в проценты
			HighPrice24h:      f(x.HighPrice24h),
			LowPrice24h:       f(x.LowPrice24h),
			Turnover24h:       f(x.Turnover24h),
			Volume24h:         f(x.Volume24h),
			Bid1Price:         f(x.Bid1Price),
			Ask1Price:         f(x.Ask1Price),
			FundingRate:       f(x.FundingRate),
			OpenInterest:      f(x.OpenInterest),
			OpenInterestValue: f(x.OpenInterestValue),
		}
	}

	return out, nil
}
