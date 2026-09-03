// internal/analysis/service.go
package analysis

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"universal-bybit-screener/config"
	"universal-bybit-screener/internal/bybit"
	"universal-bybit-screener/internal/indicators"
	"universal-bybit-screener/internal/strategies"
	"universal-bybit-screener/internal/structure"
	"universal-bybit-screener/models"
)

type Service struct {
	client     *bybit.Client
	cfg        config.Config
	strategy   strategies.Strategy
	klineCache *bybit.KlineCache
	obCache    *bybit.OrderBookCache
	wsStream   *bybit.PublicWSStream
	isWarmedUp bool
	mu         sync.Mutex
}

func NewService(c *bybit.Client, cfg config.Config, s strategies.Strategy) *Service {
	ob := bybit.NewOrderBookCache()
	kc := bybit.NewKlineCache()
	ws := bybit.NewPublicWSStream(ob, kc)

	return &Service{
		client:     c,
		cfg:        cfg,
		strategy:   s,
		klineCache: kc,
		obCache:    ob,
		wsStream:   ws,
	}
}

func (s *Service) Run(ctx context.Context) (models.ScreeningResult, error) {
	inst, err := s.client.Instruments(ctx)
	if err != nil {
		return models.ScreeningResult{}, fmt.Errorf("REST instruments fetch failed: %w", err)
	}

	tickers, err := s.client.Tickers(ctx)
	if err != nil {
		return models.ScreeningResult{}, fmt.Errorf("REST tickers fetch failed: %w", err)
	}

	type pair struct {
		i models.Instrument
		t models.Ticker
	}
	filtered := make([]pair, 0)
	for _, i := range inst {
		t, ok := tickers[i.Symbol]
		if !ok || t.LastPrice <= 0 || t.LastPrice > s.cfg.Filters.MaxPrice || t.Turnover24h < s.cfg.Filters.MinTurnover24h {
			continue
		}
		if spreadPct(t) > s.cfg.Filters.MaxGridSpreadPct {
			continue
		}
		filtered = append(filtered, pair{i, t})
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].t.Turnover24h > filtered[j].t.Turnover24h
	})

	if len(filtered) > s.cfg.Filters.PreselectCandidates {
		filtered = filtered[:s.cfg.Filters.PreselectCandidates]
	}

	symbols := make([]string, len(filtered))
	for i, p := range filtered {
		symbols[i] = p.i.Symbol
	}

	s.mu.Lock()
	if !s.isWarmedUp {
		log.Printf("[WS WARMUP] Cold start: Fetching REST Klines warmup for %d tickers...", len(symbols))
		s.warmupKlinesREST(ctx, symbols)

		log.Printf("[WS START] Starting async Public WS Stream (OrderBooks + Klines)...")
		if err := s.wsStream.Start(ctx, symbols); err != nil {
			s.mu.Unlock()
			return models.ScreeningResult{}, fmt.Errorf("WS stream start failed: %w", err)
		}

		s.isWarmedUp = true
		s.mu.Unlock()

		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return models.ScreeningResult{}, ctx.Err()
		}
	} else {
		s.mu.Unlock()
	}

	results := make([]models.Candidate, 0, len(filtered))
	var resMu sync.Mutex
	sem := make(chan struct{}, s.cfg.Concurrency)
	var wg sync.WaitGroup

	for _, p := range filtered {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			cand, e := s.analyzeWS(ctx, p.i, p.t)
			if e != nil {
				log.Printf("[WARN] Symbol %s analysis skipped: %v", p.i.Symbol, e)
				return
			}

			resMu.Lock()
			results = append(results, cand)
			resMu.Unlock()
		}()
	}
	wg.Wait()

	if ctx.Err() != nil && len(results) == 0 {
		return models.ScreeningResult{}, fmt.Errorf("analysis execution timeout/cancelled: %w", ctx.Err())
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Strategies[s.strategy.Name()].Score > results[j].Strategies[s.strategy.Name()].Score
	})

	if len(results) > s.cfg.Filters.TopCandidates {
		results = results[:s.cfg.Filters.TopCandidates]
	}

	return models.ScreeningResult{
		GeneratedAt: time.Now().UTC(),
		Strategy:    s.strategy.Name(),
		Prompt:      BuildAIPrompt(s.strategy.Name()),
		Filters:     s.cfg.Filters,
		Candidates:  results,
	}, nil
}

func (s *Service) warmupKlinesREST(ctx context.Context, symbols []string) {
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, sym := range symbols {
		sym := sym
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if c15, err := s.client.Klines(ctx, sym, "15", s.cfg.Analysis.KlineLimit15m); err == nil {
				s.klineCache.Warmup(sym, "15", c15)
			}
			if c60, err := s.client.Klines(ctx, sym, "60", s.cfg.Analysis.KlineLimit1h); err == nil {
				s.klineCache.Warmup(sym, "60", c60)
			}
			if c240, err := s.client.Klines(ctx, sym, "240", s.cfg.Analysis.KlineLimit4h); err == nil {
				s.klineCache.Warmup(sym, "240", c240)
			}
		}()
	}
	wg.Wait()
}

func (s *Service) analyzeWS(ctx context.Context, inst models.Instrument, t models.Ticker) (models.Candidate, error) {
	c15 := s.klineCache.Get(inst.Symbol, "15")
	c60 := s.klineCache.Get(inst.Symbol, "60")
	c240 := s.klineCache.Get(inst.Symbol, "240")

	if len(c15) < 20 || len(c60) < 20 || len(c240) < 20 {
		return models.Candidate{}, fmt.Errorf("insufficient WS kline history for %s", inst.Symbol)
	}

	oiCh := make(chan []models.OpenInterestPoint, 1)
	fundCh := make(chan []models.FundingPoint, 1)

	go func() {
		o, _ := s.client.OpenInterest(ctx, inst.Symbol, "1h", s.cfg.Analysis.OpenInterestLimit)
		oiCh <- o
	}()
	go func() {
		f, _ := s.client.Funding(ctx, inst.Symbol, s.cfg.Analysis.FundingLimit)
		fundCh <- f
	}()

	oi, fund := <-oiCh, <-fundCh
	book := s.obCache.GetMetrics(inst.Symbol)

	ind := models.Indicators{
		RSI15m:        indicators.RSI(c15, 14),
		RSI1h:         indicators.RSI(c60, 14),
		RSI4h:         indicators.RSI(c240, 14),
		ATR15m:        indicators.ATR(c15, 14),
		ATR1h:         indicators.ATR(c60, 14),
		ATR4h:         indicators.ATR(c240, 14),
		VolumeRatio1h: indicators.VolumeRatio(c60, 20),
		VolumeTrend1h: indicators.VolumeTrend(c60, 5, 20),
	}
	if t.LastPrice > 0 {
		ind.ATR1hPct = ind.ATR1h / t.LastPrice * 100
		ind.ATR4hPct = ind.ATR4h / t.LastPrice * 100
	}

	structures := map[string]models.Structure{
		"15m": structure.Analyze(c15, 2, 5),
		"1h":  structure.Analyze(c60, 2, 5),
		"4h":  structure.Analyze(c240, 2, 5),
	}

	levels := structure.ApplyATR(structure.Levels(structures["1h"], t.LastPrice), ind.ATR1h, t.LastPrice)

	der := models.Derivatives{
		FundingRate:  t.FundingRate,
		OpenInterest: t.OpenInterest,
		SpreadPct:    spreadPct(t),
	}
	der.FundingAvg24h = fundingAverage24h(fund)
	der.FundingAvg = der.FundingAvg24h

	nOI := len(oi)
	if nOI >= 4 && oi[nOI-4].OpenInterest > 0 {
		der.OpenInterestChange = (oi[nOI-1].OpenInterest/oi[nOI-4].OpenInterest - 1) * 100
	}

	var c models.Candidate
	c.Symbol = inst.Symbol
	c.Market.Price = t.LastPrice
	c.Market.Change24h = t.Price24hPcnt
	c.Market.Change3d = changeN(c60, 72)
	c.Market.Change7d = changeN(c60, 168)
	c.Market.Turnover24h = t.Turnover24h
	c.Market.Volume24h = t.Volume24h
	c.Market.SpreadPct = der.SpreadPct
	c.Indicators = ind
	c.Structure = structures
	c.Levels = levels
	c.Derivatives = der
	c.OrderBook = book
	c.Strategies = make(map[string]models.StrategyResult, len(strategies.Names()))

	for _, name := range strategies.Names() {
		st, e := strategies.New(name)
		if e != nil {
			return models.Candidate{}, e
		}
		c.Strategies[name] = st.Evaluate(&c)
	}

	return c, nil
}

func spreadPct(t models.Ticker) float64 {
	if t.LastPrice <= 0 || t.Bid1Price <= 0 || t.Ask1Price <= 0 {
		return 0
	}
	return (t.Ask1Price - t.Bid1Price) / t.LastPrice * 100
}

func fundingAverage24h(points []models.FundingPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	latest := points[0].Time
	for _, p := range points[1:] {
		if p.Time.After(latest) {
			latest = p.Time
		}
	}
	cutoff := latest.Add(-24 * time.Hour)
	sum := 0.0
	count := 0
	for _, p := range points {
		if !p.Time.Before(cutoff) && !p.Time.After(latest) {
			sum += p.Rate
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func changeN(c []models.Candle, n int) float64 {
	if len(c) < 2 {
		return 0
	}
	start := len(c) - 1 - n
	if start < 0 {
		start = 0
	}
	if c[start].Close == 0 {
		return 0
	}
	return (c[len(c)-1].Close/c[start].Close - 1) * 100
}

func BuildAIPrompt(strategy string) string {
	return fmt.Sprintf(`Анализ скрининга Bybit V5 для стратегии "%s".`, strategy)
}
