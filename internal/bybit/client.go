package bybit

import (
	"candidate-screener/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client — минимальный public REST-клиент Bybit V5 для массового скриннинга.
type Client struct {
	base string
	http *http.Client
}

// New создаёт клиент без API-ключей: Screener использует только публичные market endpoints.
func New(base string, timeout time.Duration) *Client {
	return &Client{base: base, http: &http.Client{Timeout: timeout}}
}

type envelope struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+"?"+q.Encode(), nil)
	if e != nil {
		return e
	}
	r, e := c.http.Do(req)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		return fmt.Errorf("Bybit HTTP %s", r.Status)
	}
	var env envelope
	if e = json.NewDecoder(r.Body).Decode(&env); e != nil {
		return e
	}
	if env.RetCode != 0 {
		return fmt.Errorf("Bybit retCode=%d: %s", env.RetCode, env.RetMsg)
	}
	return json.Unmarshal(env.Result, out)
}

// Instruments возвращает активные linear USDT perpetual инструменты и торговые ограничения.
func (c *Client) Instruments(ctx context.Context) ([]domain.Instrument, error) {
	var res struct {
		List []struct {
			Symbol, Status, ContractType, QuoteCoin string
			PriceFilter                             struct{ TickSize string }
			LotSizeFilter                           struct{ QtyStep, MinOrderQty, MinNotionalValue string }
		} `json:"list"`
		Next string `json:"nextPageCursor"`
	}
	out := []domain.Instrument{}
	cursor := ""
	for {
		q := url.Values{"category": {"linear"}, "limit": {"1000"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		if e := c.get(ctx, "/v5/market/instruments-info", q, &res); e != nil {
			return nil, e
		}
		for _, x := range res.List {
			if x.Status == "Trading" && x.QuoteCoin == "USDT" && (x.ContractType == "LinearPerpetual" || x.ContractType == "") {
				out = append(out, domain.Instrument{Symbol: x.Symbol, Status: x.Status, TickSize: num(x.PriceFilter.TickSize), QtyStep: num(x.LotSizeFilter.QtyStep), MinOrderQty: num(x.LotSizeFilter.MinOrderQty), MinNotional: num(x.LotSizeFilter.MinNotionalValue)})
			}
		}
		cursor = res.Next
		if cursor == "" {
			break
		}
	}
	return out, nil
}

// Tickers получает текущие тикеры всех linear инструментов одним массовым запросом.
func (c *Client) Tickers(ctx context.Context) (map[string]domain.Ticker, error) {
	var res struct {
		List []struct{ Symbol, LastPrice, Turnover24h, Volume24h, FundingRate, OpenInterest string } `json:"list"`
	}
	if e := c.get(ctx, "/v5/market/tickers", url.Values{"category": {"linear"}}, &res); e != nil {
		return nil, e
	}
	m := map[string]domain.Ticker{}
	for _, x := range res.List {
		m[x.Symbol] = domain.Ticker{Symbol: x.Symbol, LastPrice: num(x.LastPrice), Turnover24h: num(x.Turnover24h), Volume24h: num(x.Volume24h), FundingRate: num(x.FundingRate), OpenInterest: num(x.OpenInterest)}
	}
	return m, nil
}

// Klines загружает последние свечи заданного интервала и возвращает их по времени от старых к новым.
func (c *Client) Klines(ctx context.Context, symbol, interval string, limit int) ([]domain.Candle, error) {
	var res struct {
		List [][]string `json:"list"`
	}
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(limit)}}
	if e := c.get(ctx, "/v5/market/kline", q, &res); e != nil {
		return nil, e
	}
	out := make([]domain.Candle, 0, len(res.List))
	for i := len(res.List) - 1; i >= 0; i-- {
		x := res.List[i]
		if len(x) < 7 {
			continue
		}
		ms, _ := strconv.ParseInt(x[0], 10, 64)
		out = append(out, domain.Candle{OpenTime: time.UnixMilli(ms).UTC(), Open: num(x[1]), High: num(x[2]), Low: num(x[3]), Close: num(x[4]), Volume: num(x[5]), Turnover: num(x[6])})
	}
	return out, nil
}
func num(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
