// Package bybit содержит небольшой публичный REST-клиент Bybit V5.
// API-ключ не требуется: анализатор читает только общедоступные рыночные данные.
package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

type Config struct {
	BaseURL string
	Timeout time.Duration
}
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient создаёт HTTP-клиент с заданным базовым URL и таймаутом.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.bybit.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &Client{baseURL: cfg.BaseURL, http: &http.Client{Timeout: cfg.Timeout}}
}

type envelope struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

// get выполняет GET-запрос и распаковывает стандартный envelope Bybit V5.
func (c *Client) get(ctx context.Context, path string, q url.Values, dst any) error {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "crypto-coin-analyzer-deep/2.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bybit HTTP %d: %s", resp.StatusCode, string(body))
	}
	var env envelope
	if err = json.Unmarshal(body, &env); err != nil {
		return err
	}
	if env.RetCode != 0 {
		return fmt.Errorf("Bybit retCode=%d: %s", env.RetCode, env.RetMsg)
	}
	if err = json.Unmarshal(env.Result, dst); err != nil {
		return fmt.Errorf("разбор result %s: %w", path, err)
	}
	return nil
}

func f64(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
func i64(s string) int64   { v, _ := strconv.ParseInt(s, 10, 64); return v }

type Ticker struct {
	Symbol                                                                                                                                              string
	LastPrice, Price24hPct, High24h, Low24h, Turnover24h, Volume24h, FundingRate, OpenInterest, OpenInterestValue, BidPrice, AskPrice, BidSize, AskSize float64
}

// Ticker проверяет существование linear-символа и возвращает текущий тикер.
func (c *Client) Ticker(ctx context.Context, symbol string) (Ticker, error) {
	var r struct {
		List []struct {
			Symbol       string `json:"symbol"`
			LastPrice    string `json:"lastPrice"`
			Price24hPcnt string `json:"price24hPcnt"`
			High         string `json:"highPrice24h"`
			Low          string `json:"lowPrice24h"`
			Turnover     string `json:"turnover24h"`
			Volume       string `json:"volume24h"`
			Funding      string `json:"fundingRate"`
			OI           string `json:"openInterest"`
			OIV          string `json:"openInterestValue"`
			Bid          string `json:"bid1Price"`
			Ask          string `json:"ask1Price"`
			BidSize      string `json:"bid1Size"`
			AskSize      string `json:"ask1Size"`
		} `json:"list"`
	}
	if err := c.get(ctx, "/v5/market/tickers", url.Values{"category": {"linear"}, "symbol": {symbol}}, &r); err != nil {
		return Ticker{}, err
	}
	if len(r.List) == 0 {
		return Ticker{}, fmt.Errorf("символ %s не найден среди Bybit linear", symbol)
	}
	x := r.List[0]
	return Ticker{x.Symbol, f64(x.LastPrice), f64(x.Price24hPcnt) * 100, f64(x.High), f64(x.Low), f64(x.Turnover), f64(x.Volume), f64(x.Funding), f64(x.OI), f64(x.OIV), f64(x.Bid), f64(x.Ask), f64(x.BidSize), f64(x.AskSize)}, nil
}

type Candle struct {
	Time     time.Time `json:"time"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
	Turnover float64   `json:"turnover"`
}

// KlinesRange загружает свечи с пагинацией назад во времени. Это устраняет лимит 1000 свечей одного запроса.
func (c *Client) KlinesRange(ctx context.Context, symbol, interval string, count int) ([]Candle, error) {
	if count < 1 {
		return nil, nil
	}
	if count > 10000 {
		count = 10000
	}
	out := make([]Candle, 0, count)
	var end int64
	for len(out) < count {
		lim := count - len(out)
		if lim > 1000 {
			lim = 1000
		}
		q := url.Values{"category": {"linear"}, "symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(lim)}}
		if end > 0 {
			q.Set("end", strconv.FormatInt(end, 10))
		}
		var r struct {
			List [][]string `json:"list"`
		}
		if err := c.get(ctx, "/v5/market/kline", q, &r); err != nil {
			return nil, err
		}
		if len(r.List) == 0 {
			break
		}
		oldest := int64(1<<63 - 1)
		for _, row := range r.List {
			if len(row) < 7 {
				continue
			}
			ms := i64(row[0])
			if ms < oldest {
				oldest = ms
			}
			out = append(out, Candle{time.UnixMilli(ms).UTC(), f64(row[1]), f64(row[2]), f64(row[3]), f64(row[4]), f64(row[5]), f64(row[6])})
		}
		if oldest == int64(1<<63-1) || len(r.List) < lim {
			break
		}
		end = oldest - 1
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if len(out) > count {
		out = out[len(out)-count:]
	}
	return out, nil
}

type PriceCandle struct {
	Time                   time.Time `json:"time"`
	Open, High, Low, Close float64
}

// priceKlinesRange — общий загрузчик mark/index свечей.
func (c *Client) priceKlinesRange(ctx context.Context, path, symbol, interval string, count int) ([]PriceCandle, error) {
	if count < 1 {
		return nil, nil
	}
	if count > 5000 {
		count = 5000
	}
	out := make([]PriceCandle, 0, count)
	var end int64
	for len(out) < count {
		lim := count - len(out)
		if lim > 1000 {
			lim = 1000
		}
		q := url.Values{"category": {"linear"}, "symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(lim)}}
		if end > 0 {
			q.Set("end", strconv.FormatInt(end, 10))
		}
		var r struct {
			List [][]string `json:"list"`
		}
		if err := c.get(ctx, path, q, &r); err != nil {
			return nil, err
		}
		if len(r.List) == 0 {
			break
		}
		old := int64(1<<63 - 1)
		for _, row := range r.List {
			if len(row) < 5 {
				continue
			}
			ms := i64(row[0])
			if ms < old {
				old = ms
			}
			out = append(out, PriceCandle{time.UnixMilli(ms).UTC(), f64(row[1]), f64(row[2]), f64(row[3]), f64(row[4])})
		}
		if old == int64(1<<63-1) || len(r.List) < lim {
			break
		}
		end = old - 1
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if len(out) > count {
		out = out[len(out)-count:]
	}
	return out, nil
}

// MarkKlinesRange возвращает историю mark price.
func (c *Client) MarkKlinesRange(ctx context.Context, symbol, interval string, count int) ([]PriceCandle, error) {
	return c.priceKlinesRange(ctx, "/v5/market/mark-price-kline", symbol, interval, count)
}

// IndexKlinesRange возвращает историю index price.
func (c *Client) IndexKlinesRange(ctx context.Context, symbol, interval string, count int) ([]PriceCandle, error) {
	return c.priceKlinesRange(ctx, "/v5/market/index-price-kline", symbol, interval, count)
}

type Funding struct {
	Time time.Time `json:"time"`
	Rate float64   `json:"rate"`
}

// Funding загружает последние ставки funding.
func (c *Client) Funding(ctx context.Context, symbol string, limit int) ([]Funding, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	var r struct {
		List []struct {
			Rate string `json:"fundingRate"`
			TS   string `json:"fundingRateTimestamp"`
		} `json:"list"`
	}
	if err := c.get(ctx, "/v5/market/funding/history", url.Values{"category": {"linear"}, "symbol": {symbol}, "limit": {strconv.Itoa(limit)}}, &r); err != nil {
		return nil, err
	}
	o := make([]Funding, 0, len(r.List))
	for _, x := range r.List {
		o = append(o, Funding{time.UnixMilli(i64(x.TS)).UTC(), f64(x.Rate)})
	}
	sort.Slice(o, func(i, j int) bool { return o[i].Time.Before(o[j].Time) })
	return o, nil
}

type OpenInterest struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// OpenInterest загружает историю OI с cursor-пагинацией.
func (c *Client) OpenInterest(ctx context.Context, symbol, interval string, count int) ([]OpenInterest, error) {
	if count < 1 {
		return nil, nil
	}
	if count > 2000 {
		count = 2000
	}
	out := make([]OpenInterest, 0, count)
	cursor := ""
	for len(out) < count {
		lim := count - len(out)
		if lim > 200 {
			lim = 200
		}
		q := url.Values{"category": {"linear"}, "symbol": {symbol}, "intervalTime": {interval}, "limit": {strconv.Itoa(lim)}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var r struct {
			List []struct {
				Value string `json:"openInterest"`
				TS    string `json:"timestamp"`
			} `json:"list"`
			Cursor string `json:"nextPageCursor"`
		}
		if err := c.get(ctx, "/v5/market/open-interest", q, &r); err != nil {
			return nil, err
		}
		for _, x := range r.List {
			out = append(out, OpenInterest{time.UnixMilli(i64(x.TS)).UTC(), f64(x.Value)})
		}
		if r.Cursor == "" || len(r.List) == 0 {
			break
		}
		cursor = r.Cursor
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if len(out) > count {
		out = out[len(out)-count:]
	}
	return out, nil
}

type LongShort struct {
	Time       time.Time `json:"time"`
	LongRatio  float64   `json:"long_ratio"`
	ShortRatio float64   `json:"short_ratio"`
}

// LongShort загружает историю долей long/short аккаунтов.
func (c *Client) LongShort(ctx context.Context, symbol, period string, count int) ([]LongShort, error) {
	if count < 1 {
		return nil, nil
	}
	if count > 3000 {
		count = 3000
	}
	out := make([]LongShort, 0, count)
	cursor := ""
	for len(out) < count {
		lim := count - len(out)
		if lim > 500 {
			lim = 500
		}
		q := url.Values{"category": {"linear"}, "symbol": {symbol}, "period": {period}, "limit": {strconv.Itoa(lim)}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var r struct {
			List []struct {
				Buy  string `json:"buyRatio"`
				Sell string `json:"sellRatio"`
				TS   string `json:"timestamp"`
			} `json:"list"`
			Cursor string `json:"nextPageCursor"`
		}
		if err := c.get(ctx, "/v5/market/account-ratio", q, &r); err != nil {
			return nil, err
		}
		for _, x := range r.List {
			out = append(out, LongShort{time.UnixMilli(i64(x.TS)).UTC(), f64(x.Buy), f64(x.Sell)})
		}
		if r.Cursor == "" || len(r.List) == 0 {
			break
		}
		cursor = r.Cursor
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if len(out) > count {
		out = out[len(out)-count:]
	}
	return out, nil
}

type OrderBook struct {
	BestBid      float64 `json:"best_bid"`
	BestAsk      float64 `json:"best_ask"`
	BidNotional  float64 `json:"bid_notional"`
	AskNotional  float64 `json:"ask_notional"`
	ImbalancePct float64 `json:"imbalance_pct"`
	Ratio        float64 `json:"bid_ask_ratio"`
	Levels       int     `json:"levels"`
}

// OrderBook получает текущий snapshot глубины стакана.
func (c *Client) OrderBook(ctx context.Context, symbol string, limit int) (OrderBook, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 1000 {
		limit = 1000
	}
	var r struct {
		Bids [][]string `json:"b"`
		Asks [][]string `json:"a"`
	}
	if err := c.get(ctx, "/v5/market/orderbook", url.Values{"category": {"linear"}, "symbol": {symbol}, "limit": {strconv.Itoa(limit)}}, &r); err != nil {
		return OrderBook{}, err
	}
	sum := func(rows [][]string) float64 {
		z := 0.0
		for _, x := range rows {
			if len(x) >= 2 {
				z += f64(x[0]) * f64(x[1])
			}
		}
		return z
	}
	b, a := sum(r.Bids), sum(r.Asks)
	o := OrderBook{BidNotional: b, AskNotional: a, Levels: len(r.Bids)}
	if len(r.Bids) > 0 {
		o.BestBid = f64(r.Bids[0][0])
	}
	if len(r.Asks) > 0 {
		o.BestAsk = f64(r.Asks[0][0])
	}
	if a > 0 {
		o.Ratio = b / a
	}
	if a+b > 0 {
		o.ImbalancePct = (b - a) / (b + a) * 100
	}
	return o, nil
}

type Trade struct {
	Time  time.Time `json:"time"`
	Price float64   `json:"price"`
	Size  float64   `json:"size"`
	Side  string    `json:"side"`
}

// RecentTrades получает до 1000 последних публичных сделок для оценки taker-flow/CVD snapshot.
func (c *Client) RecentTrades(ctx context.Context, symbol string, limit int) ([]Trade, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 1000 {
		limit = 1000
	}
	var r struct {
		List []struct {
			Price string `json:"price"`
			Size  string `json:"size"`
			Side  string `json:"side"`
			Time  string `json:"time"`
		} `json:"list"`
	}
	if err := c.get(ctx, "/v5/market/recent-trade", url.Values{"category": {"linear"}, "symbol": {symbol}, "limit": {strconv.Itoa(limit)}}, &r); err != nil {
		return nil, err
	}
	o := make([]Trade, 0, len(r.List))
	for _, x := range r.List {
		o = append(o, Trade{time.UnixMilli(i64(x.Time)).UTC(), f64(x.Price), f64(x.Size), x.Side})
	}
	sort.Slice(o, func(i, j int) bool { return o[i].Time.Before(o[j].Time) })
	return o, nil
}
