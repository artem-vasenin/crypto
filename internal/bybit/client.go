// Публичный REST-клиент Bybit V5.
// Авторизация здесь не нужна: используются только публичные market endpoints.
package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	return &Client{baseURL: cfg.BaseURL, http: &http.Client{Timeout: timeout}}
}

type envelope struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

func (c *Client) get(ctx context.Context, path string, q url.Values, dst any) error {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "crypto-coin-analyzer/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("чтение ответа %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bybit HTTP %d: %s", resp.StatusCode, string(body))
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("разбор ответа %s: %w", path, err)
	}
	if env.RetCode != 0 {
		return fmt.Errorf("Bybit retCode=%d: %s", env.RetCode, env.RetMsg)
	}
	if err := json.Unmarshal(env.Result, dst); err != nil {
		return fmt.Errorf("разбор result %s: %w", path, err)
	}
	return nil
}

type Ticker struct {
	Symbol            string
	LastPrice         float64
	PrevPrice24h      float64
	Price24hPct       float64
	High24h           float64
	Low24h            float64
	Turnover24h       float64
	Volume24h         float64
	FundingRate       float64
	OpenInterest      float64
	OpenInterestValue float64
	BidPrice          float64
	AskPrice          float64
	BidSize           float64
	AskSize           float64
}

func (c *Client) Ticker(ctx context.Context, symbol string) (Ticker, error) {
	var r struct {
		List []struct {
			Symbol            string `json:"symbol"`
			LastPrice         string `json:"lastPrice"`
			PrevPrice24h      string `json:"prevPrice24h"`
			Price24hPcnt      string `json:"price24hPcnt"`
			HighPrice24h      string `json:"highPrice24h"`
			LowPrice24h       string `json:"lowPrice24h"`
			Turnover24h       string `json:"turnover24h"`
			Volume24h         string `json:"volume24h"`
			FundingRate       string `json:"fundingRate"`
			OpenInterest      string `json:"openInterest"`
			OpenInterestValue string `json:"openInterestValue"`
			Bid1Price         string `json:"bid1Price"`
			Ask1Price         string `json:"ask1Price"`
			Bid1Size          string `json:"bid1Size"`
			Ask1Size          string `json:"ask1Size"`
		} `json:"list"`
	}
	q := url.Values{"category": {"linear"}, "symbol": {symbol}}
	if err := c.get(ctx, "/v5/market/tickers", q, &r); err != nil {
		return Ticker{}, err
	}
	if len(r.List) == 0 {
		return Ticker{}, fmt.Errorf("символ %s не найден на Bybit linear", symbol)
	}
	x := r.List[0]
	f := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	return Ticker{Symbol: x.Symbol, LastPrice: f(x.LastPrice), PrevPrice24h: f(x.PrevPrice24h), Price24hPct: f(x.Price24hPcnt) * 100,
		High24h: f(x.HighPrice24h), Low24h: f(x.LowPrice24h), Turnover24h: f(x.Turnover24h), Volume24h: f(x.Volume24h), FundingRate: f(x.FundingRate),
		OpenInterest: f(x.OpenInterest), OpenInterestValue: f(x.OpenInterestValue), BidPrice: f(x.Bid1Price), AskPrice: f(x.Ask1Price), BidSize: f(x.Bid1Size), AskSize: f(x.Ask1Size)}, nil
}

type Candle struct {
	Time                                     time.Time
	Open, High, Low, Close, Volume, Turnover float64
}

func (c *Client) Klines(ctx context.Context, symbol, interval string, limit int) ([]Candle, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 1000 {
		limit = 1000
	}
	var r struct {
		List [][]string `json:"list"`
	}
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(limit)}}
	if err := c.get(ctx, "/v5/market/kline", q, &r); err != nil {
		return nil, err
	}
	out := make([]Candle, 0, len(r.List))
	f := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	for _, row := range r.List {
		if len(row) < 7 {
			continue
		}
		ms, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			continue
		}
		out = append(out, Candle{Time: time.UnixMilli(ms).UTC(), Open: f(row[1]), High: f(row[2]), Low: f(row[3]), Close: f(row[4]), Volume: f(row[5]), Turnover: f(row[6])})
	}
	// Bybit возвращает свечи от новых к старым; для индикаторов удобнее старые -> новые.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

type Funding struct {
	Time time.Time
	Rate float64
}

func (c *Client) Funding(ctx context.Context, symbol string, limit int) ([]Funding, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	var r struct {
		List []struct {
			Symbol    string `json:"symbol"`
			Rate      string `json:"fundingRate"`
			Timestamp string `json:"fundingRateTimestamp"`
		} `json:"list"`
	}
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "limit": {strconv.Itoa(limit)}}
	if err := c.get(ctx, "/v5/market/funding/history", q, &r); err != nil {
		return nil, err
	}
	out := make([]Funding, 0, len(r.List))
	for _, x := range r.List {
		ms, _ := strconv.ParseInt(x.Timestamp, 10, 64)
		rate, _ := strconv.ParseFloat(x.Rate, 64)
		out = append(out, Funding{Time: time.UnixMilli(ms).UTC(), Rate: rate})
	}
	return out, nil
}

type OpenInterest struct {
	Time  time.Time
	Value float64
}

func (c *Client) OpenInterest(ctx context.Context, symbol, interval string, limit int) ([]OpenInterest, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	var r struct {
		List []struct {
			OpenInterest string `json:"openInterest"`
			Timestamp    string `json:"timestamp"`
		} `json:"list"`
	}
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "intervalTime": {interval}, "limit": {strconv.Itoa(limit)}}
	if err := c.get(ctx, "/v5/market/open-interest", q, &r); err != nil {
		return nil, err
	}
	out := make([]OpenInterest, 0, len(r.List))
	for _, x := range r.List {
		ms, _ := strconv.ParseInt(x.Timestamp, 10, 64)
		v, _ := strconv.ParseFloat(x.OpenInterest, 64)
		out = append(out, OpenInterest{Time: time.UnixMilli(ms).UTC(), Value: v})
	}
	return out, nil
}

type LongShort struct {
	Time                                                   time.Time
	BuyRatio, SellRatio, BuyAccountRatio, SellAccountRatio float64
}

func (c *Client) LongShort(ctx context.Context, symbol, period string, limit int) ([]LongShort, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	var r struct {
		List []struct {
			BuyRatio         string `json:"buyRatio"`
			SellRatio        string `json:"sellRatio"`
			BuyAccountRatio  string `json:"buyRatioByAcct"`
			SellAccountRatio string `json:"sellRatioByAcct"`
			Timestamp        string `json:"timestamp"`
		} `json:"list"`
	}
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "period": {period}, "limit": {strconv.Itoa(limit)}}
	if err := c.get(ctx, "/v5/market/account-ratio", q, &r); err != nil {
		return nil, err
	}
	out := make([]LongShort, 0, len(r.List))
	for _, x := range r.List {
		ms, _ := strconv.ParseInt(x.Timestamp, 10, 64)
		br, _ := strconv.ParseFloat(x.BuyRatio, 64)
		sr, _ := strconv.ParseFloat(x.SellRatio, 64)
		ba, _ := strconv.ParseFloat(x.BuyAccountRatio, 64)
		sa, _ := strconv.ParseFloat(x.SellAccountRatio, 64)
		out = append(out, LongShort{Time: time.UnixMilli(ms).UTC(), BuyRatio: br, SellRatio: sr, BuyAccountRatio: ba, SellAccountRatio: sa})
	}
	return out, nil
}

type OrderBook struct {
	BidNotional, AskNotional float64
	BestBid, BestAsk         float64
	ImbalancePct             float64
	Ratio                    float64
	Levels                   int
}

func (c *Client) OrderBook(ctx context.Context, symbol string, limit int) (OrderBook, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	var r struct {
		Bids [][]string `json:"b"`
		Asks [][]string `json:"a"`
	}
	q := url.Values{"category": {"linear"}, "symbol": {symbol}, "limit": {strconv.Itoa(limit)}}
	if err := c.get(ctx, "/v5/market/orderbook", q, &r); err != nil {
		return OrderBook{}, err
	}
	sum := func(rows [][]string) float64 {
		var s float64
		for _, row := range rows {
			if len(row) >= 2 {
				p, _ := strconv.ParseFloat(row[0], 64)
				q, _ := strconv.ParseFloat(row[1], 64)
				s += p * q
			}
		}
		return s
	}
	b, a := sum(r.Bids), sum(r.Asks)
	ratio := 0.0
	if a > 0 {
		ratio = b / a
	}
	imb := 0.0
	if b+a > 0 {
		imb = (b - a) / (b + a) * 100
	}
	ob := OrderBook{BidNotional: b, AskNotional: a, ImbalancePct: imb, Ratio: ratio, Levels: len(r.Bids)}
	if len(r.Bids) > 0 {
		ob.BestBid, _ = strconv.ParseFloat(r.Bids[0][0], 64)
	}
	if len(r.Asks) > 0 {
		ob.BestAsk, _ = strconv.ParseFloat(r.Asks[0][0], 64)
	}
	return ob, nil
}
