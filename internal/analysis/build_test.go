package analysis

import (
	"context"
	"testing"
	"time"

	"crypto-coin-analyzer/internal/bybit"
)

type fakeAPI struct{}

func (fakeAPI) Ticker(context.Context, string) (bybit.Ticker, error) {
	return bybit.Ticker{Symbol: "TESTUSDT", LastPrice: 100, Price24hPct: 2, Turnover24h: 10_000_000, Volume24h: 100_000, FundingRate: 0.0001, OpenInterest: 1_000_000, BidPrice: 99.99, AskPrice: 100.01}, nil
}
func (fakeAPI) Klines(context.Context, string, string, int) ([]bybit.Candle, error) {
	out := make([]bybit.Candle, 220)
	for i := range out {
		v := 100 + float64(i)*0.01
		out[i] = bybit.Candle{Time: time.Unix(int64(i*60), 0), Open: v, High: v + 0.5, Low: v - 0.5, Close: v + 0.1, Volume: float64(100 + i)}
	}
	return out, nil
}
func (fakeAPI) Funding(context.Context, string, int) ([]bybit.Funding, error) {
	return []bybit.Funding{{Rate: 0.0001}}, nil
}
func (fakeAPI) OpenInterest(context.Context, string, string, int) ([]bybit.OpenInterest, error) {
	return []bybit.OpenInterest{{Value: 100}, {Value: 110}}, nil
}
func (fakeAPI) LongShort(context.Context, string, string, int) ([]bybit.LongShort, error) {
	return []bybit.LongShort{{BuyRatio: 0.55, SellRatio: 0.45}}, nil
}
func (fakeAPI) OrderBook(context.Context, string, int) (bybit.OrderBook, error) {
	return bybit.OrderBook{BidNotional: 1000, AskNotional: 900, ImbalancePct: 5.26, Ratio: 1.11, Levels: 200}, nil
}
func TestBuild(t *testing.T) {
	r, err := Build(context.Background(), fakeAPI{}, "TESTUSDT", 1)
	if err != nil {
		t.Fatal(err)
	}
	if r.Symbol != "TESTUSDT" || r.Market.Price != 100 {
		t.Fatalf("bad report: %+v", r)
	}
	if r.AIInstructions.Task == "" {
		t.Fatal("missing AI instructions")
	}
}
