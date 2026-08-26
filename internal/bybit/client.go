package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type responseEnvelope struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

type Ticker struct {
	Symbol        string `json:"symbol"`
	LastPrice     string `json:"lastPrice"`
	PrevPrice24h  string `json:"prevPrice24h"`
	Price24hPcnt  string `json:"price24hPcnt"`
	HighPrice24h  string `json:"highPrice24h"`
	LowPrice24h   string `json:"lowPrice24h"`
	PrevPrice1h   string `json:"prevPrice1h"`
	Turnover24h   string `json:"turnover24h"`
	Volume24h     string `json:"volume24h"`
	FundingRate   string `json:"fundingRate"`
	OpenInterest  string `json:"openInterest"`
	OpenInterestV string `json:"openInterestValue"`
}

type Kline struct {
	StartTime int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Turnover  float64
}

type OrderBookLevel struct {
	Price float64
	Size  float64
}

type OrderBook struct {
	Bids []OrderBookLevel
	Asks []OrderBookLevel
	Time int64
}

type PublicTrade struct {
	Price float64
	Size  float64
	Side  string
	Time  int64
}

type OIRecord struct {
	Time  int64
	Value float64
}

type FundingRecord struct {
	Time int64
	Rate float64
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if envelope.RetCode != 0 {
		return fmt.Errorf("Bybit error %d: %s", envelope.RetCode, envelope.RetMsg)
	}
	if err := json.Unmarshal(envelope.Result, out); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

func (c *Client) GetTickers(ctx context.Context, category string) ([]Ticker, error) {
	var result struct {
		List []Ticker `json:"list"`
	}
	q := url.Values{}
	q.Set("category", category)
	if err := c.get(ctx, "/v5/market/tickers", q, &result); err != nil {
		return nil, err
	}
	return result.List, nil
}

func (c *Client) GetKlines(ctx context.Context, category, symbol, interval string, limit int) ([]Kline, error) {
	var result struct {
		List [][]string `json:"list"`
	}
	q := url.Values{}
	q.Set("category", category)
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	q.Set("limit", strconv.Itoa(limit))
	if err := c.get(ctx, "/v5/market/kline", q, &result); err != nil {
		return nil, err
	}

	// Bybit returns newest-first. We reverse it so all indicator code can work
	// naturally from oldest -> newest.
	klines := make([]Kline, 0, len(result.List))
	for _, row := range result.List {
		if len(row) < 7 {
			continue
		}
		ts, err1 := strconv.ParseInt(row[0], 10, 64)
		open, err2 := strconv.ParseFloat(row[1], 64)
		high, err3 := strconv.ParseFloat(row[2], 64)
		low, err4 := strconv.ParseFloat(row[3], 64)
		closePrice, err5 := strconv.ParseFloat(row[4], 64)
		volume, err6 := strconv.ParseFloat(row[5], 64)
		turnover, err7 := strconv.ParseFloat(row[6], 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil || err7 != nil {
			continue
		}
		klines = append(klines, Kline{StartTime: ts, Open: open, High: high, Low: low, Close: closePrice, Volume: volume, Turnover: turnover})
	}
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}
	return klines, nil
}

func (c *Client) GetOrderBook(ctx context.Context, category, symbol string, limit int) (OrderBook, error) {
	var result struct {
		Bids [][]string `json:"b"`
		Asks [][]string `json:"a"`
		Time int64      `json:"ts"`
	}
	q := url.Values{}
	q.Set("category", category)
	q.Set("symbol", symbol)
	q.Set("limit", strconv.Itoa(limit))
	if err := c.get(ctx, "/v5/market/orderbook", q, &result); err != nil {
		return OrderBook{}, err
	}
	parse := func(rows [][]string) []OrderBookLevel {
		levels := make([]OrderBookLevel, 0, len(rows))
		for _, row := range rows {
			if len(row) < 2 {
				continue
			}
			p, e1 := strconv.ParseFloat(row[0], 64)
			s, e2 := strconv.ParseFloat(row[1], 64)
			if e1 == nil && e2 == nil {
				levels = append(levels, OrderBookLevel{Price: p, Size: s})
			}
		}
		return levels
	}
	return OrderBook{Bids: parse(result.Bids), Asks: parse(result.Asks), Time: result.Time}, nil
}

func (c *Client) GetRecentTrades(ctx context.Context, category, symbol string, limit int) ([]PublicTrade, error) {
	var result struct {
		List []struct {
			Price string `json:"p"`
			Size  string `json:"v"`
			Side  string `json:"S"`
			Time  string `json:"T"`
		} `json:"list"`
	}
	q := url.Values{}
	q.Set("category", category)
	q.Set("symbol", symbol)
	q.Set("limit", strconv.Itoa(limit))
	if err := c.get(ctx, "/v5/market/recent-trade", q, &result); err != nil {
		return nil, err
	}
	trades := make([]PublicTrade, 0, len(result.List))
	for _, item := range result.List {
		p, e1 := strconv.ParseFloat(item.Price, 64)
		s, e2 := strconv.ParseFloat(item.Size, 64)
		t, e3 := strconv.ParseInt(item.Time, 10, 64)
		if e1 == nil && e2 == nil && e3 == nil {
			trades = append(trades, PublicTrade{Price: p, Size: s, Side: item.Side, Time: t})
		}
	}
	return trades, nil
}

func (c *Client) GetOpenInterest(ctx context.Context, category, symbol, interval string, limit int) ([]OIRecord, error) {
	var result struct {
		List []struct {
			Time string `json:"timestamp"`
			OI   string `json:"openInterest"`
		} `json:"list"`
	}
	q := url.Values{}
	q.Set("category", category)
	q.Set("symbol", symbol)
	q.Set("intervalTime", interval)
	q.Set("limit", strconv.Itoa(limit))
	if err := c.get(ctx, "/v5/market/open-interest", q, &result); err != nil {
		return nil, err
	}
	out := make([]OIRecord, 0, len(result.List))
	for _, item := range result.List {
		t, e1 := strconv.ParseInt(item.Time, 10, 64)
		v, e2 := strconv.ParseFloat(item.OI, 64)
		if e1 == nil && e2 == nil {
			out = append(out, OIRecord{Time: t, Value: v})
		}
	}
	// API is newest-first; reverse.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (c *Client) GetFundingHistory(ctx context.Context, category, symbol string, limit int) ([]FundingRecord, error) {
	var result struct {
		List []struct {
			Rate string `json:"fundingRate"`
			Time string `json:"fundingRateTimestamp"`
		} `json:"list"`
	}
	q := url.Values{}
	q.Set("category", category)
	q.Set("symbol", symbol)
	q.Set("limit", strconv.Itoa(limit))
	if err := c.get(ctx, "/v5/market/funding/history", q, &result); err != nil {
		return nil, err
	}
	out := make([]FundingRecord, 0, len(result.List))
	for _, item := range result.List {
		r, e1 := strconv.ParseFloat(item.Rate, 64)
		t, e2 := strconv.ParseInt(item.Time, 10, 64)
		if e1 == nil && e2 == nil {
			out = append(out, FundingRecord{Time: t, Rate: r})
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
