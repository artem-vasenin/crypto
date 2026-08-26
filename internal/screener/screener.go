package screener

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"bybit-screener/internal/analysis"
	"bybit-screener/internal/bybit"
	"bybit-screener/internal/config"
	"bybit-screener/internal/indicators"
)

type Screener struct {
	Client *bybit.Client
	Config config.Config
}

type Result struct {
	GeneratedAt     time.Time           `json:"generated_at"`
	PrimaryStrategy string              `json:"primary_strategy"`
	Filters         config.FilterConfig `json:"filters"`
	Candidates      []Candidate         `json:"candidates"`
}

type Candidate struct {
	Symbol      string                             `json:"symbol"`
	Market      MarketData                         `json:"market"`
	Indicators  IndicatorData                      `json:"indicators"`
	Trend       TrendData                          `json:"trend"`
	Momentum    MomentumData                       `json:"momentum"`
	Volume      VolumeData                         `json:"volume"`
	Volatility  VolatilityData                     `json:"volatility"`
	Ranges      RangeData                          `json:"ranges"`
	Structure   map[string]indicators.Structure    `json:"structure"`
	Levels      LevelData                          `json:"levels"`
	Derivatives DerivativeData                     `json:"derivatives"`
	OrderBook   OrderBookData                      `json:"order_book"`
	Trades      TradeData                          `json:"trades"`
	BTC         BTCData                            `json:"btc_context"`
	Strategies  map[string]analysis.StrategyResult `json:"strategies"`
}

type MarketData struct {
	Price        float64 `json:"price"`
	Change24hPct float64 `json:"change_24h_pct"`
	Change3dPct  float64 `json:"change_3d_pct"`
	Change7dPct  float64 `json:"change_7d_pct"`
	Turnover24h  float64 `json:"turnover_24h"`
	Volume24h    float64 `json:"volume_24h"`
	SpreadPct    float64 `json:"spread_pct"`
}
type IndicatorData struct {
	RSI15m        float64 `json:"rsi_15m"`
	RSI1h         float64 `json:"rsi_1h"`
	RSI4h         float64 `json:"rsi_4h"`
	ATR15m        float64 `json:"atr_15m"`
	ATR1h         float64 `json:"atr_1h"`
	ATR4h         float64 `json:"atr_4h"`
	VolumeRatio1h float64 `json:"volume_ratio_1h"`
}
type TrendData struct {
	EMA20_15m        float64 `json:"ema20_15m"`
	EMA50_15m        float64 `json:"ema50_15m"`
	EMA200_15m       float64 `json:"ema200_15m"`
	EMA20_1h         float64 `json:"ema20_1h"`
	EMA50_1h         float64 `json:"ema50_1h"`
	EMA200_1h        float64 `json:"ema200_1h"`
	EMA20_4h         float64 `json:"ema20_4h"`
	EMA50_4h         float64 `json:"ema50_4h"`
	EMA200_4h        float64 `json:"ema200_4h"`
	PriceVsEMA20_1h  float64 `json:"price_vs_ema20_1h_pct"`
	PriceVsEMA50_1h  float64 `json:"price_vs_ema50_1h_pct"`
	PriceVsEMA200_1h float64 `json:"price_vs_ema200_1h_pct"`
}
type MomentumData struct {
	Change1hPct  float64 `json:"change_1h_pct"`
	Change4hPct  float64 `json:"change_4h_pct"`
	Change12hPct float64 `json:"change_12h_pct"`
	Change24hPct float64 `json:"change_24h_pct"`
	ROC1hPct     float64 `json:"roc_1h_pct"`
	ROC4hPct     float64 `json:"roc_4h_pct"`
}
type VolumeData struct {
	Volume5m       float64 `json:"volume_5m"`
	Volume15m      float64 `json:"volume_15m"`
	Volume1h       float64 `json:"volume_1h"`
	Ratio5m        float64 `json:"ratio_5m"`
	Ratio15m       float64 `json:"ratio_15m"`
	Ratio1h        float64 `json:"ratio_1h"`
	Buy5m          float64 `json:"buy_volume_5m"`
	Sell5m         float64 `json:"sell_volume_5m"`
	BuySellRatio5m float64 `json:"buy_sell_ratio_5m"`
}
type VolatilityData struct {
	ATR15mPct      float64 `json:"atr_15m_pct"`
	ATR1hPct       float64 `json:"atr_1h_pct"`
	ATR4hPct       float64 `json:"atr_4h_pct"`
	Realized24hPct float64 `json:"realized_volatility_24h_pct"`
}
type RangeItem struct {
	Low         float64 `json:"low"`
	High        float64 `json:"high"`
	RangePct    float64 `json:"range_pct"`
	PositionPct float64 `json:"current_position_pct"`
}
type RangeData struct {
	H24 RangeItem `json:"24h"`
	D3  RangeItem `json:"3d"`
	D7  RangeItem `json:"7d"`
}
type LevelData struct {
	Resistance              []float64 `json:"resistance"`
	Support                 []float64 `json:"support"`
	DistanceToResistancePct float64   `json:"distance_to_resistance_pct"`
	DistanceToSupportPct    float64   `json:"distance_to_support_pct"`
}
type DerivativeData struct {
	FundingRate           float64 `json:"funding_rate"`
	FundingAvg            float64 `json:"funding_avg"`
	FundingAvg24h         float64 `json:"funding_avg_24h"`
	FundingAvg3d          float64 `json:"funding_avg_3d"`
	FundingMin3d          float64 `json:"funding_min_3d"`
	FundingMax3d          float64 `json:"funding_max_3d"`
	FundingTrend          string  `json:"funding_trend"`
	OpenInterest          float64 `json:"open_interest"`
	OIChange1hPct         float64 `json:"open_interest_change_1h_pct"`
	OIChange4hPct         float64 `json:"open_interest_change_4h_pct"`
	OIChange24hPct        float64 `json:"open_interest_change_24h_pct"`
	OIChange3dPct         float64 `json:"open_interest_change_3d_pct"`
	OpenInterestChangePct float64 `json:"open_interest_change_pct"`
	PriceOIState1h        string  `json:"price_oi_state_1h"`
	PriceOIState4h        string  `json:"price_oi_state_4h"`
	SpreadPct             float64 `json:"spread_pct"`
}
type OrderBookData struct {
	BestBid        float64            `json:"best_bid"`
	BestAsk        float64            `json:"best_ask"`
	BidDepth       map[string]float64 `json:"bid_depth"`
	AskDepth       map[string]float64 `json:"ask_depth"`
	Imbalance      map[string]float64 `json:"imbalance"`
	LargestBidWall float64            `json:"largest_bid_wall"`
	LargestAskWall float64            `json:"largest_ask_wall"`
}
type TradeData struct {
	BuyVolume5m     float64 `json:"buy_volume_5m"`
	SellVolume5m    float64 `json:"sell_volume_5m"`
	BuySellRatio5m  float64 `json:"buy_sell_ratio_5m"`
	BuyVolume15m    float64 `json:"buy_volume_15m"`
	SellVolume15m   float64 `json:"sell_volume_15m"`
	BuySellRatio15m float64 `json:"buy_sell_ratio_15m"`
	Aggression5m    string  `json:"aggression_5m"`
	Aggression15m   string  `json:"aggression_15m"`
}
type BTCData struct {
	Change1hPct    float64 `json:"change_1h_pct"`
	Change4hPct    float64 `json:"change_4h_pct"`
	Trend          string  `json:"trend"`
	Correlation1h  float64 `json:"correlation_1h"`
	Correlation4h  float64 `json:"correlation_4h"`
	Correlation24h float64 `json:"correlation_24h"`
}

func (s *Screener) Run(ctx context.Context, strategy string) (Result, error) {
	tickers, err := s.Client.GetTickers(ctx, s.Config.Bybit.Category)
	if err != nil {
		return Result{}, err
	}
	candidates := make([]bybit.Ticker, 0)
	for _, t := range tickers {
		if t.Symbol == "" || t.Symbol == "BTCUSDT" {
			continue
		}
		price := parseFloat(t.LastPrice)
		turnover := parseFloat(t.Turnover24h)
		if price <= 0 || price > s.Config.Filters.MaxPrice || turnover < s.Config.Filters.MinTurnover24h {
			continue
		}
		candidates = append(candidates, t)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return parseFloat(candidates[i].Price24hPcnt) > parseFloat(candidates[j].Price24hPcnt)
	})
	prelimit := s.Config.Filters.TopCandidates * 3
	if prelimit < 30 {
		prelimit = 30
	}
	if len(candidates) > prelimit {
		candidates = candidates[:prelimit]
	}

	type item struct {
		c   Candidate
		err error
	}
	jobs := make(chan bybit.Ticker)
	results := make(chan item, len(candidates))
	var wg sync.WaitGroup
	workers := s.Config.Analysis.Concurrency
	if workers > len(candidates) {
		workers = len(candidates)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range jobs {
				c, err := s.analyze(ctx, t)
				results <- item{c, err}
			}
		}()
	}
	go func() {
		for _, t := range candidates {
			select {
			case jobs <- t:
			case <-ctx.Done():
				break
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	out := make([]Candidate, 0)
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		out = append(out, r.c)
	}
	if len(out) == 0 && firstErr != nil {
		return Result{}, firstErr
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Strategies[strategy].Score > out[j].Strategies[strategy].Score })
	if len(out) > s.Config.Filters.TopCandidates {
		out = out[:s.Config.Filters.TopCandidates]
	}
	return Result{GeneratedAt: time.Now().UTC(), PrimaryStrategy: strategy, Filters: s.Config.Filters, Candidates: out}, nil
}

func (s *Screener) analyze(ctx context.Context, t bybit.Ticker) (Candidate, error) {
	symbol := t.Symbol
	p := parseFloat(t.LastPrice)
	k15, err := s.Client.GetKlines(ctx, s.Config.Bybit.Category, symbol, "15", s.Config.Analysis.KlineLimit15m)
	if err != nil {
		return Candidate{}, err
	}
	k1, err := s.Client.GetKlines(ctx, s.Config.Bybit.Category, symbol, "60", s.Config.Analysis.KlineLimit1h)
	if err != nil {
		return Candidate{}, err
	}
	k4, err := s.Client.GetKlines(ctx, s.Config.Bybit.Category, symbol, "240", s.Config.Analysis.KlineLimit4h)
	if err != nil {
		return Candidate{}, err
	}
	k5, err := s.Client.GetKlines(ctx, s.Config.Bybit.Category, symbol, "5", s.Config.Analysis.KlineLimit5m)
	if err != nil {
		return Candidate{}, err
	}
	book, err := s.Client.GetOrderBook(ctx, s.Config.Bybit.Category, symbol, s.Config.Analysis.OrderbookLimit)
	if err != nil {
		return Candidate{}, err
	}
	trades, err := s.Client.GetRecentTrades(ctx, s.Config.Bybit.Category, symbol, s.Config.Analysis.RecentTradesLimit)
	if err != nil {
		return Candidate{}, err
	}
	oi, err := s.Client.GetOpenInterest(ctx, s.Config.Bybit.Category, symbol, s.Config.Analysis.OIInterval, s.Config.Analysis.OILimit)
	if err != nil {
		return Candidate{}, err
	}
	funding, err := s.Client.GetFundingHistory(ctx, s.Config.Bybit.Category, symbol, s.Config.Analysis.FundingLimit)
	if err != nil {
		return Candidate{}, err
	}

	r15, r1, r4 := indicators.RSI(k15, 14), indicators.RSI(k1, 14), indicators.RSI(k4, 14)
	a15, a1, a4 := indicators.ATR(k15, 14), indicators.ATR(k1, 14), indicators.ATR(k4, 14)
	s15, s1, s4 := indicators.FindPivots(k15, s.Config.Analysis.PivotWindow), indicators.FindPivots(k1, s.Config.Analysis.PivotWindow), indicators.FindPivots(k4, s.Config.Analysis.PivotWindow)
	resistance := append([]float64{}, pivotPrices(s15.PivotHighs)...)
	resistance = append(resistance, pivotPrices(s1.PivotHighs)...)
	resistance = append(resistance, pivotPrices(s4.PivotHighs)...)
	support := append([]float64{}, pivotPrices(s15.PivotLows)...)
	support = append(support, pivotPrices(s1.PivotLows)...)
	support = append(support, pivotPrices(s4.PivotLows)...)
	resistance = uniqueLevels(resistance)
	support = uniqueLevels(support)

	volRatio1 := indicators.VolumeRatio(k1, s.Config.Analysis.VolumeBaselineBars)
	v5 := sumRecentVolume(k5, 1)
	v15 := sumRecentVolume(k5, 3)
	v1 := sumRecentVolume(k5, 12)
	range24 := makeRange(k1, 24)
	range3d := makeRange(k1, 72)
	range7d := makeRange(k1, 168)
	bookData, bookImb := analyzeBook(book, p, s.Config.Analysis.OrderbookDepthPct)
	tradeData, tradeImb := analyzeTrades(trades, time.Now())
	der := analyzeDerivatives(oi, funding, indicators.ChangePct(k1, 1), indicators.ChangePct(k1, 4), indicators.ChangePct(k1, 24), indicators.ChangePct(k1, 72))
	der.SpreadPct = bookSpread(book)
	der.OpenInterestChangePct = der.OIChange24hPct
	btc := s.analyzeBTC(ctx, k5)

	c := Candidate{Symbol: symbol}
	c.Market = MarketData{Price: p, Change24hPct: parseFloat(t.Price24hPcnt) * 100, Change3dPct: indicators.ChangePct(k1, 72), Change7dPct: indicators.ChangePct(k1, 168), Turnover24h: parseFloat(t.Turnover24h), Volume24h: parseFloat(t.Volume24h), SpreadPct: bookSpread(book)}
	c.Indicators = IndicatorData{RSI15m: r15, RSI1h: r1, RSI4h: r4, ATR15m: a15, ATR1h: a1, ATR4h: a4, VolumeRatio1h: volRatio1}
	c.Trend = TrendData{EMA20_15m: indicators.EMA(k15, 20), EMA50_15m: indicators.EMA(k15, 50), EMA200_15m: indicators.EMA(k15, 200), EMA20_1h: indicators.EMA(k1, 20), EMA50_1h: indicators.EMA(k1, 50), EMA200_1h: indicators.EMA(k1, 200), EMA20_4h: indicators.EMA(k4, 20), EMA50_4h: indicators.EMA(k4, 50), EMA200_4h: indicators.EMA(k4, 200)}
	c.Trend.PriceVsEMA20_1h = pctFrom(p, c.Trend.EMA20_1h)
	c.Trend.PriceVsEMA50_1h = pctFrom(p, c.Trend.EMA50_1h)
	c.Trend.PriceVsEMA200_1h = pctFrom(p, c.Trend.EMA200_1h)
	c.Momentum = MomentumData{Change1hPct: indicators.ChangePct(k1, 1), Change4hPct: indicators.ChangePct(k1, 4), Change12hPct: indicators.ChangePct(k1, 12), Change24hPct: indicators.ChangePct(k1, 24), ROC1hPct: indicators.ChangePct(k1, 1), ROC4hPct: indicators.ChangePct(k1, 4)}
	c.Volume = VolumeData{Volume5m: v5, Volume15m: v15, Volume1h: v1, Ratio5m: indicators.VolumeRatio(k5, 48), Ratio15m: indicators.VolumeRatio(k15, 48), Ratio1h: volRatio1, Buy5m: tradeData.BuyVolume5m, Sell5m: tradeData.SellVolume5m, BuySellRatio5m: tradeData.BuySellRatio5m}
	c.Volatility = VolatilityData{ATR15mPct: pct(a15, p), ATR1hPct: pct(a1, p), ATR4hPct: pct(a4, p), Realized24hPct: indicators.Std(indicators.Returns(k1)) * 100}
	c.Ranges = RangeData{H24: range24, D3: range3d, D7: range7d}
	c.Structure = map[string]indicators.Structure{"15m": s15, "1h": s1, "4h": s4}
	c.Levels = LevelData{Resistance: resistance, Support: support}
	if x := indicators.NearestAbove(p, resistance); x > 0 {
		c.Levels.DistanceToResistancePct = pct(x-p, p)
	}
	if x := indicators.NearestBelow(p, support); x > 0 {
		c.Levels.DistanceToSupportPct = pct(p-x, p)
	}
	c.Derivatives = der
	c.OrderBook = bookData
	c.Trades = tradeData
	c.BTC = btc
	c.Strategies = analysis.Score(p, c.Market.Change24hPct, c.Market.Change3dPct, r15, r1, r4, a15, a1, a4, volRatio1, s15, s1, s4, resistance, support, der.FundingRate, der.FundingAvg24h, der.OIChange24hPct, bookImb, tradeImb)
	return c, nil
}

func (s *Screener) analyzeBTC(ctx context.Context, alt []bybit.Kline) BTCData {
	// Use 5-minute BTC candles so 1h/4h/24h correlations have enough observations.
	btc, err := s.Client.GetKlines(ctx, s.Config.Bybit.Category, "BTCUSDT", "5", 300)
	if err != nil {
		return BTCData{}
	}
	out := BTCData{Change1hPct: indicators.ChangePct(btc, 1), Change4hPct: indicators.ChangePct(btc, 4)}
	if out.Change4hPct > 1 {
		out.Trend = "bullish"
	} else if out.Change4hPct < -1 {
		out.Trend = "bearish"
	} else {
		out.Trend = "neutral"
	}
	a := indicators.Returns(alt)
	b := indicators.Returns(btc)
	out.Correlation1h = indicators.Pearson(tail(a, 12), tail(b, 12))
	out.Correlation4h = indicators.Pearson(tail(a, 48), tail(b, 48))
	out.Correlation24h = indicators.Pearson(tail(a, 288), tail(b, 288))
	return out
}

func parseFloat(s string) float64 { var v float64; fmt.Sscanf(s, "%f", &v); return v }
func pct(v, base float64) float64 {
	if base == 0 {
		return 0
	}
	return v / base * 100
}
func pctFrom(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a - b) / b * 100
}
func pivotPrices(p []indicators.Pivot) []float64 {
	out := make([]float64, 0, len(p))
	for _, x := range p {
		out = append(out, x.Price)
	}
	return out
}
func uniqueLevels(in []float64) []float64 {
	out := []float64{}
	for _, x := range in {
		found := false
		for _, y := range out {
			if pct(x-y, x) < 0.25 && pct(x-y, x) > -0.25 {
				found = true
				break
			}
		}
		if !found {
			out = append(out, x)
		}
	}
	return out
}
func sumRecentVolume(k []bybit.Kline, n int) float64 {
	s := 0.0
	start := len(k) - n
	if start < 0 {
		start = 0
	}
	for i := start; i < len(k); i++ {
		s += k[i].Volume
	}
	return s
}
func makeRange(k []bybit.Kline, bars int) RangeItem {
	lo, hi, pos := indicators.Range(k, bars)
	return RangeItem{Low: lo, High: hi, RangePct: pct(hi-lo, lo), PositionPct: pos}
}
func tail(v []float64, n int) []float64 {
	if len(v) < 2 {
		return v
	}
	bars := n
	if bars > len(v)-1 {
		bars = len(v) - 1
	}
	return v[len(v)-bars:]
}

func bookSpread(b bybit.OrderBook) float64 {
	if len(b.Bids) == 0 || len(b.Asks) == 0 {
		return 0
	}
	bid := b.Bids[0].Price
	ask := b.Asks[0].Price
	if bid == 0 {
		return 0
	}
	return (ask - bid) / bid * 100
}
func analyzeBook(b bybit.OrderBook, price float64, depths []float64) (OrderBookData, float64) {
	d := OrderBookData{BidDepth: map[string]float64{}, AskDepth: map[string]float64{}, Imbalance: map[string]float64{}}
	if len(b.Bids) > 0 {
		d.BestBid = b.Bids[0].Price
	}
	if len(b.Asks) > 0 {
		d.BestAsk = b.Asks[0].Price
	}
	for _, dp := range depths {
		key := fmt.Sprintf("%.1f_pct", dp)
		bid, ask := 0.0, 0.0
		for _, x := range b.Bids {
			if x.Price >= price*(1-dp/100) {
				bid += x.Size * x.Price
			}
		}
		for _, x := range b.Asks {
			if x.Price <= price*(1+dp/100) {
				ask += x.Size * x.Price
			}
		}
		d.BidDepth[key] = bid
		d.AskDepth[key] = ask
		if bid+ask > 0 {
			d.Imbalance[key] = (bid - ask) / (bid + ask)
		}
	}
	for _, x := range b.Bids {
		v := x.Size * x.Price
		if v > d.LargestBidWall {
			d.LargestBidWall = v
		}
	}
	for _, x := range b.Asks {
		v := x.Size * x.Price
		if v > d.LargestAskWall {
			d.LargestAskWall = v
		}
	}
	return d, d.Imbalance[fmt.Sprintf("%.1f_pct", depths[0])]
}
func analyzeTrades(tr []bybit.PublicTrade, now time.Time) (TradeData, float64) {
	d := TradeData{}
	cut5 := now.Add(-5 * time.Minute).UnixMilli()
	cut15 := now.Add(-15 * time.Minute).UnixMilli()
	for _, x := range tr {
		if x.Time >= cut15 {
			if x.Side == "Buy" {
				d.BuyVolume15m += x.Size * x.Price
			} else {
				d.SellVolume15m += x.Size * x.Price
			}
		}
		if x.Time >= cut5 {
			if x.Side == "Buy" {
				d.BuyVolume5m += x.Size * x.Price
			} else {
				d.SellVolume5m += x.Size * x.Price
			}
		}
	}
	if d.SellVolume5m > 0 {
		d.BuySellRatio5m = d.BuyVolume5m / d.SellVolume5m
	}
	if d.SellVolume15m > 0 {
		d.BuySellRatio15m = d.BuyVolume15m / d.SellVolume15m
	}
	d.Aggression5m = aggression(d.BuyVolume5m, d.SellVolume5m)
	d.Aggression15m = aggression(d.BuyVolume15m, d.SellVolume15m)
	den := d.BuyVolume5m + d.SellVolume5m
	if den == 0 {
		return d, 0
	}
	return d, (d.BuyVolume5m - d.SellVolume5m) / den
}
func aggression(b, s float64) string {
	if b+s == 0 {
		return "unknown"
	}
	r := (b - s) / (b + s)
	if r > 0.15 {
		return "buy_dominant"
	}
	if r < -0.15 {
		return "sell_dominant"
	}
	return "balanced"
}
func analyzeDerivatives(oi []bybit.OIRecord, fr []bybit.FundingRecord, p1, p4, p24, p3d float64) DerivativeData {
	d := DerivativeData{}
	if len(oi) > 0 {
		d.OpenInterest = oi[len(oi)-1].Value
		d.OIChange1hPct = oiChange(oi, 1*time.Hour)
		d.OIChange4hPct = oiChange(oi, 4*time.Hour)
		d.OIChange24hPct = oiChange(oi, 24*time.Hour)
		d.OIChange3dPct = oiChange(oi, 72*time.Hour)
	}
	if len(fr) > 0 {
		d.FundingRate = fr[len(fr)-1].Rate
		vals24 := fundingSince(fr, 24*time.Hour)
		vals3d := fundingSince(fr, 72*time.Hour)
		d.FundingAvg24h = indicators.Mean(vals24)
		d.FundingAvg = d.FundingAvg24h
		d.FundingAvg3d = indicators.Mean(vals3d)
		d.FundingMin3d = indicators.Min(vals3d)
		d.FundingMax3d = indicators.Max(vals3d)
		if len(fr) >= 2 {
			if fr[len(fr)-1].Rate > fr[len(fr)-2].Rate {
				d.FundingTrend = "rising"
			} else if fr[len(fr)-1].Rate < fr[len(fr)-2].Rate {
				d.FundingTrend = "falling"
			} else {
				d.FundingTrend = "flat"
			}
		}
	}
	d.PriceOIState1h = priceOI(p1, d.OIChange1hPct)
	d.PriceOIState4h = priceOI(p4, d.OIChange4hPct)
	_ = p24
	_ = p3d
	return d
}
func oiChange(v []bybit.OIRecord, d time.Duration) float64 {
	if len(v) < 2 {
		return 0
	}
	now := v[len(v)-1]
	target := now.Time - d.Milliseconds()
	var old *bybit.OIRecord
	for i := len(v) - 2; i >= 0; i-- {
		if v[i].Time <= target {
			tmp := v[i]
			old = &tmp
			break
		}
	}
	if old == nil || old.Value == 0 {
		return 0
	}
	return (now.Value - old.Value) / old.Value * 100
}
func fundingSince(v []bybit.FundingRecord, d time.Duration) []float64 {
	if len(v) == 0 {
		return nil
	}
	cut := v[len(v)-1].Time - d.Milliseconds()
	out := []float64{}
	for _, x := range v {
		if x.Time >= cut {
			out = append(out, x.Rate)
		}
	}
	return out
}
func priceOI(priceChange, oiChange float64) string {
	if priceChange > 0.2 && oiChange > 0.2 {
		return "price_up_oi_up"
	}
	if priceChange > 0.2 && oiChange < -0.2 {
		return "price_up_oi_down"
	}
	if priceChange < -0.2 && oiChange > 0.2 {
		return "price_down_oi_up"
	}
	if priceChange < -0.2 && oiChange < -0.2 {
		return "price_down_oi_down"
	}
	return "mixed"
}
