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
	client   *bybit.Client
	cfg      config.Config
	strategy strategies.Strategy

	klineCache *bybit.KlineCache
	obCache    *bybit.OrderBookCache
	wsStream   *bybit.PublicWSStream

	mu            sync.Mutex
	warmedSymbols map[string]bool
}

func NewService(c *bybit.Client, cfg config.Config, s strategies.Strategy) *Service {
	ob := bybit.NewOrderBookCache()
	kc := bybit.NewKlineCache()
	return &Service{
		client:        c,
		cfg:           cfg,
		strategy:      s,
		klineCache:    kc,
		obCache:       ob,
		wsStream:      bybit.NewPublicWSStream(ob, kc, cfg.Analysis.OrderBookLimit),
		warmedSymbols: make(map[string]bool),
	}
}

func (s *Service) Run(ctx context.Context) (models.ScreeningResult, error) {
	instruments, err := s.client.Instruments(ctx)
	if err != nil {
		return models.ScreeningResult{}, fmt.Errorf("REST instruments fetch failed: %w", err)
	}

	tickers, err := s.client.Tickers(ctx)
	if err != nil {
		return models.ScreeningResult{}, fmt.Errorf("REST tickers fetch failed: %w", err)
	}

	type pair struct {
		instrument models.Instrument
		ticker     models.Ticker
	}

	filtered := make([]pair, 0, len(instruments))
	for _, instrument := range instruments {
		ticker, ok := tickers[instrument.Symbol]
		if !ok || instrument.Status != "Trading" || ticker.LastPrice <= 0 {
			continue
		}
		if s.cfg.Filters.MaxPrice > 0 && ticker.LastPrice > s.cfg.Filters.MaxPrice {
			continue
		}
		if ticker.Turnover24h < s.cfg.Filters.MinTurnover24h {
			continue
		}

		spread := spreadPct(ticker)
		if spread <= 0 || spread > s.cfg.Filters.MaxGridSpreadPct {
			continue
		}

		filtered = append(filtered, pair{instrument: instrument, ticker: ticker})
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ticker.Turnover24h > filtered[j].ticker.Turnover24h
	})
	if len(filtered) > s.cfg.Filters.PreselectCandidates {
		filtered = filtered[:s.cfg.Filters.PreselectCandidates]
	}

	symbols := make([]string, 0, len(filtered))
	for _, p := range filtered {
		symbols = append(symbols, p.instrument.Symbol)
	}

	if len(symbols) == 0 {
		return models.ScreeningResult{
			GeneratedAt: time.Now().UTC(),
			Strategy:    s.strategy.Name(),
			Prompt:      BuildAIPrompt(s.strategy.Name()),
			Filters:     s.cfg.Filters,
			Candidates:  []models.Candidate{},
		}, nil
	}

	if err := s.ensureMarketData(ctx, symbols); err != nil {
		return models.ScreeningResult{}, err
	}

	results := make([]models.Candidate, 0, len(filtered))
	var resultMu sync.Mutex
	sem := make(chan struct{}, maxInt(1, s.cfg.Concurrency))
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

			candidate, err := s.analyze(ctx, p.instrument, p.ticker)
			if err != nil {
				log.Printf("[WARN] %s skipped: %v", p.instrument.Symbol, err)
				return
			}

			resultMu.Lock()
			results = append(results, candidate)
			resultMu.Unlock()
		}()
	}
	wg.Wait()

	if ctx.Err() != nil {
		return models.ScreeningResult{}, ctx.Err()
	}

	sort.Slice(results, func(i, j int) bool {
		left := results[i].Strategies[s.strategy.Name()]
		right := results[j].Strategies[s.strategy.Name()]
		if left.Decision.Eligible != right.Decision.Eligible {
			return left.Decision.Eligible
		}
		if left.Scores.EntryQuality.Score != right.Scores.EntryQuality.Score {
			return left.Scores.EntryQuality.Score > right.Scores.EntryQuality.Score
		}
		return left.Scores.AssetQuality.Score > right.Scores.AssetQuality.Score
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

func (s *Service) ensureMarketData(ctx context.Context, symbols []string) error {
	s.mu.Lock()
	s.wsStream.AddSymbols(symbols)
	if err := s.wsStream.Start(ctx, symbols); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("public WS start failed: %w", err)
	}

	toWarm := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if !s.warmedSymbols[symbol] || !s.hasEnoughKlines(symbol) {
			toWarm = append(toWarm, symbol)
		}
	}
	s.mu.Unlock()

	if len(toWarm) > 0 {
		s.warmupKlinesREST(ctx, toWarm)
		s.mu.Lock()
		for _, symbol := range toWarm {
			if s.hasEnoughKlines(symbol) {
				s.warmedSymbols[symbol] = true
			}
		}
		s.mu.Unlock()
	}

	// New subscriptions need a short opportunity to receive an order-book snapshot.
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if s.allOrderBooksReady(symbols) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("order book warmup timeout: not all selected symbols have fresh snapshots")
		case <-ticker.C:
		}
	}
}

func (s *Service) hasEnoughKlines(symbol string) bool {
	min := func(interval string, limit int) bool {
		if limit <= 0 {
			return true
		}
		return len(s.klineCache.Get(symbol, interval)) >= minInt(limit, 20)
	}
	return min("5", s.cfg.Analysis.KlineLimit5m) &&
		min("15", s.cfg.Analysis.KlineLimit15m) &&
		min("30", s.cfg.Analysis.KlineLimit30m) &&
		min("60", s.cfg.Analysis.KlineLimit1h) &&
		min("240", s.cfg.Analysis.KlineLimit4h)
}

func (s *Service) allOrderBooksReady(symbols []string) bool {
	maxAge := time.Duration(s.cfg.Analysis.MaxDataAgeSeconds) * time.Second
	for _, symbol := range symbols {
		if s.obCache.Age(symbol) > maxAge {
			return false
		}
	}
	return true
}

func (s *Service) warmupKlinesREST(ctx context.Context, symbols []string) {
	sem := make(chan struct{}, minInt(10, maxInt(1, s.cfg.Concurrency)))
	var wg sync.WaitGroup

	for _, symbol := range symbols {
		symbol := symbol
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			s.fetchWarmup(ctx, symbol, "5", s.cfg.Analysis.KlineLimit5m)
			s.fetchWarmup(ctx, symbol, "15", s.cfg.Analysis.KlineLimit15m)
			s.fetchWarmup(ctx, symbol, "30", s.cfg.Analysis.KlineLimit30m)
			s.fetchWarmup(ctx, symbol, "60", s.cfg.Analysis.KlineLimit1h)
			s.fetchWarmup(ctx, symbol, "240", s.cfg.Analysis.KlineLimit4h)
		}()
	}
	wg.Wait()
}

func (s *Service) fetchWarmup(ctx context.Context, symbol, interval string, limit int) {
	if limit <= 0 {
		return
	}
	candles, err := s.client.Klines(ctx, symbol, interval, limit)
	if err != nil {
		log.Printf("[WARN] %s %s warmup failed: %v", symbol, interval, err)
		return
	}
	s.klineCache.Warmup(symbol, interval, candles)
}

func (s *Service) analyze(ctx context.Context, inst models.Instrument, ticker models.Ticker) (models.Candidate, error) {
	c5 := closedCandles(s.klineCache.Get(inst.Symbol, "5"))
	c15 := closedCandles(s.klineCache.Get(inst.Symbol, "15"))
	c30 := closedCandles(s.klineCache.Get(inst.Symbol, "30"))
	c60 := closedCandles(s.klineCache.Get(inst.Symbol, "60"))
	c240 := closedCandles(s.klineCache.Get(inst.Symbol, "240"))

	if len(c5) < 20 || len(c15) < 20 || len(c60) < 20 || len(c240) < 20 {
		return models.Candidate{}, fmt.Errorf("insufficient closed kline history")
	}

	oiCh := make(chan []models.OpenInterestPoint, 1)
	fundCh := make(chan []models.FundingPoint, 1)

	go func() {
		oi, _ := s.client.OpenInterest(ctx, inst.Symbol, "1h", s.cfg.Analysis.OpenInterestLimit)
		oiCh <- oi
	}()
	go func() {
		funding, _ := s.client.Funding(ctx, inst.Symbol, s.cfg.Analysis.FundingLimit)
		fundCh <- funding
	}()

	oi, funding := <-oiCh, <-fundCh
	if len(oi) < 4 || len(funding) == 0 {
		return models.Candidate{}, fmt.Errorf("missing derivative history")
	}

	book := s.obCache.GetMetrics(inst.Symbol)
	if book.Levels < 2 {
		return models.Candidate{}, fmt.Errorf("order book snapshot is unavailable")
	}

	ind := models.Indicators{
		RSI5m:         indicators.RSI(c5, 14),
		RSI15m:        indicators.RSI(c15, 14),
		RSI1h:         indicators.RSI(c60, 14),
		RSI4h:         indicators.RSI(c240, 14),
		ATR5m:         indicators.ATR(c5, 14),
		ATR15m:        indicators.ATR(c15, 14),
		ATR1h:         indicators.ATR(c60, 14),
		ATR4h:         indicators.ATR(c240, 14),
		VolumeRatio1h: indicators.VolumeRatio(c60, 20),
		VolumeTrend1h: indicators.VolumeTrend(c60, 5, 20),
	}

	if ticker.LastPrice > 0 {
		ind.ATR5mPct = ind.ATR5m / ticker.LastPrice * 100
		ind.ATR1hPct = ind.ATR1h / ticker.LastPrice * 100
		ind.ATR4hPct = ind.ATR4h / ticker.LastPrice * 100
	}

	structures := map[string]models.Structure{
		"5m":  structure.Analyze(c5, 2, 5),
		"15m": structure.Analyze(c15, 2, 5),
		"30m": structure.Analyze(c30, 2, 5),
		"1h":  structure.Analyze(c60, 2, 5),
		"4h":  structure.Analyze(c240, 2, 5),
	}

	levels := structure.ApplyATR(
		structure.Levels(structures["1h"], ticker.LastPrice),
		ind.ATR1h,
		ticker.LastPrice,
	)

	der := models.Derivatives{
		FundingRate:    ticker.FundingRate,
		OpenInterest:   ticker.OpenInterest,
		SpreadPct:      spreadPct(ticker),
		DataAgeSeconds: s.obCache.Age(inst.Symbol).Seconds(),
	}
	der.FundingAvg24h = fundingAverage24h(funding)
	der.FundingAvg = der.FundingAvg24h

	if oi[len(oi)-4].OpenInterest <= 0 {
		return models.Candidate{}, fmt.Errorf("invalid open interest history")
	}
	der.OpenInterestChange = (oi[len(oi)-1].OpenInterest/oi[len(oi)-4].OpenInterest - 1) * 100

	var candidate models.Candidate
	candidate.Symbol = inst.Symbol
	candidate.Market.Price = ticker.LastPrice
	candidate.Market.Change24h = ticker.Price24hPcnt
	candidate.Market.Change3d = changeN(c60, 72)
	candidate.Market.Change7d = changeN(c60, 168)
	candidate.Market.Turnover24h = ticker.Turnover24h
	candidate.Market.Volume24h = ticker.Volume24h
	candidate.Market.SpreadPct = der.SpreadPct
	candidate.Indicators = ind
	candidate.Structure = structures
	candidate.Levels = levels
	candidate.Derivatives = der
	candidate.OrderBook = book
	candidate.Strategies = make(map[string]models.StrategyResult, len(strategies.Names()))

	for _, name := range strategies.Names() {
		st, err := strategies.New(name)
		if err != nil {
			return models.Candidate{}, err
		}
		candidate.Strategies[name] = st.Evaluate(&candidate)
	}

	return candidate, nil
}

func closedCandles(candles []models.Candle) []models.Candle {
	if len(candles) <= 1 {
		return nil
	}
	out := make([]models.Candle, len(candles)-1)
	copy(out, candles[:len(candles)-1])
	return out
}

func fundingAverage24h(points []models.FundingPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	latest := points[0].Time
	for _, point := range points[1:] {
		if point.Time.After(latest) {
			latest = point.Time
		}
	}
	cutoff := latest.Add(-24 * time.Hour)
	sum := 0.0
	count := 0
	for _, point := range points {
		if !point.Time.Before(cutoff) && !point.Time.After(latest) {
			sum += point.Rate
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func changeN(candles []models.Candle, n int) float64 {
	if len(candles) < 2 {
		return 0
	}
	start := len(candles) - 1 - n
	if start < 0 {
		start = 0
	}
	if candles[start].Close <= 0 {
		return 0
	}
	return (candles[len(candles)-1].Close/candles[start].Close - 1) * 100
}

func spreadPct(ticker models.Ticker) float64 {
	if ticker.LastPrice <= 0 || ticker.Bid1Price <= 0 || ticker.Ask1Price <= 0 ||
		ticker.Ask1Price < ticker.Bid1Price {
		return 0
	}
	return (ticker.Ask1Price - ticker.Bid1Price) / ticker.LastPrice * 100
}

func BuildAIPrompt(strategy string) string {
	return fmt.Sprintf(`Анализ скрининга Bybit V5 для стратегии "%s". Данные без подтвержденного значения считать неизвестными; не придумывать отсутствующие поля.`, strategy)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
