package analysis

import (
	"context"
	"fmt"
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
}

func NewService(c *bybit.Client, cfg config.Config, s strategies.Strategy) *Service {
	return &Service{client: c, cfg: cfg, strategy: s}
}

// Run строит pipeline: сначала дешёвый фильтр ликвидности, затем глубокий
// анализ ограниченного пула. В отличие от старой версии предварительный пул
// не сортируется по росту 24h, иначе Neutral Grid теряет боковые монеты.
func (s *Service) Run(ctx context.Context) (models.ScreeningResult, error) {
	inst, err := s.client.Instruments(ctx)
	if err != nil {
		return models.ScreeningResult{}, fmt.Errorf("instruments: %w", err)
	}
	tickers, err := s.client.Tickers(ctx)
	if err != nil {
		return models.ScreeningResult{}, fmt.Errorf("tickers: %w", err)
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
		// Для Grid уже на дешёвом этапе можно убрать заведомо дорогой спред.
		// Long/Short оставляем без этого ограничения: им ликвидность важна,
		// но требования к издержкам входа/выхода у них другие.
		if isGridStrategy(s.strategy.Name()) && spreadPct(t) > s.cfg.Filters.MaxGridSpreadPct {
			continue
		}
		filtered = append(filtered, pair{i, t})
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].t.Turnover24h > filtered[j].t.Turnover24h })
	if len(filtered) > s.cfg.Filters.PreselectCandidates {
		filtered = filtered[:s.cfg.Filters.PreselectCandidates]
	}

	results := make([]models.Candidate, 0, len(filtered))
	var mu sync.Mutex
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
			cand, e := s.analyze(ctx, p.i, p.t)
			if e != nil {
				return
			}
			mu.Lock()
			results = append(results, cand)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if ctx.Err() != nil && len(results) == 0 {
		return models.ScreeningResult{}, fmt.Errorf("analysis timeout: %w", ctx.Err())
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Strategies[s.strategy.Name()].Score > results[j].Strategies[s.strategy.Name()].Score
	})
	if len(results) > s.cfg.Filters.TopCandidates {
		results = results[:s.cfg.Filters.TopCandidates]
	}
	return models.ScreeningResult{GeneratedAt: time.Now().UTC(), Strategy: s.strategy.Name(), Filters: s.cfg.Filters, Candidates: results}, nil
}

func isGridStrategy(name string) bool {
	return name == "short-grid" || name == "long-grid" || name == "neutral-grid"
}

func spreadPct(t models.Ticker) float64 {
	if t.LastPrice <= 0 || t.Bid1Price <= 0 || t.Ask1Price <= 0 {
		return 0
	}
	return (t.Ask1Price - t.Bid1Price) / t.LastPrice * 100
}

// analyze собирает независимые данные параллельно и строит единый snapshot,
// который затем без повторных API-запросов оценивают все пять стратегий.
func (s *Service) analyze(ctx context.Context, inst models.Instrument, t models.Ticker) (models.Candidate, error) {
	type res struct {
		c   []models.Candle
		err error
	}
	ch15, ch1, ch4 := make(chan res, 1), make(chan res, 1), make(chan res, 1)
	oiCh := make(chan []models.OpenInterestPoint, 1)
	fundCh := make(chan []models.FundingPoint, 1)
	bookCh := make(chan models.OrderBookMetrics, 1)
	go func() {
		c, e := s.client.Klines(ctx, inst.Symbol, "15", s.cfg.Analysis.KlineLimit15m)
		ch15 <- res{c, e}
	}()
	go func() { c, e := s.client.Klines(ctx, inst.Symbol, "60", s.cfg.Analysis.KlineLimit1h); ch1 <- res{c, e} }()
	go func() {
		c, e := s.client.Klines(ctx, inst.Symbol, "240", s.cfg.Analysis.KlineLimit4h)
		ch4 <- res{c, e}
	}()
	go func() {
		o, _ := s.client.OpenInterest(ctx, inst.Symbol, "1h", s.cfg.Analysis.OpenInterestLimit)
		oiCh <- o
	}()
	go func() { f, _ := s.client.Funding(ctx, inst.Symbol, s.cfg.Analysis.FundingLimit); fundCh <- f }()
	go func() { b, _ := s.client.OrderBook(ctx, inst.Symbol, s.cfg.Analysis.OrderBookLimit); bookCh <- b }()
	a, b, d := <-ch15, <-ch1, <-ch4
	oi, fund, book := <-oiCh, <-fundCh, <-bookCh
	if a.err != nil || b.err != nil || d.err != nil {
		return models.Candidate{}, fmt.Errorf("kline error")
	}
	md := models.MarketData{Instrument: inst, Ticker: t, Candles15m: a.c, Candles1h: b.c, Candles4h: d.c, OpenInterest: oi, Funding: fund, OrderBook: book}
	ind := models.Indicators{
		RSI15m:        indicators.RSI(a.c, 14),
		RSI1h:         indicators.RSI(b.c, 14),
		RSI4h:         indicators.RSI(d.c, 14),
		ATR15m:        indicators.ATR(a.c, 14),
		ATR1h:         indicators.ATR(b.c, 14),
		ATR4h:         indicators.ATR(d.c, 14),
		VolumeRatio1h: indicators.VolumeRatio(b.c, 20),
		VolumeTrend1h: indicators.VolumeTrend(b.c, 5, 20),
	}
	if t.LastPrice > 0 {
		ind.ATR1hPct = ind.ATR1h / t.LastPrice * 100
		ind.ATR4hPct = ind.ATR4h / t.LastPrice * 100
	}
	structures := map[string]models.Structure{
		"15m": structure.Analyze(a.c, 2, 5),
		"1h":  structure.Analyze(b.c, 2, 5),
		"4h":  structure.Analyze(d.c, 2, 5),
	}
	levels := structure.ApplyATR(structure.Levels(structures["1h"], t.LastPrice), ind.ATR1h, t.LastPrice)
	der := models.Derivatives{FundingRate: t.FundingRate, OpenInterest: t.OpenInterest, SpreadPct: spreadPct(t)}
	der.FundingAvg24h = fundingAverage24h(fund)
	// Сохраняем старое поле как совместимый alias, но теперь оно означает именно среднее за 24h.
	der.FundingAvg = der.FundingAvg24h
	if len(oi) >= 2 && oi[0].OpenInterest > 0 {
		der.OpenInterestChange = (oi[len(oi)-1].OpenInterest/oi[0].OpenInterest - 1) * 100
	}
	var c models.Candidate
	c.Symbol = inst.Symbol
	c.Market.Price = t.LastPrice
	c.Market.Change24h = t.Price24hPcnt
	c.Market.Change3d = changeN(b.c, 72)
	c.Market.Change7d = changeN(b.c, 168)
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
		c.Strategies[name] = st.Evaluate(md, ind, structures, levels)
	}
	return c, nil
}

// fundingAverage24h берёт только историю внутри 24 часов относительно
// последней доступной точки. Это важно, потому что интервал funding у разных
// контрактов может отличаться и может динамически меняться.
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

// changeN считает процентное изменение N свечей назад; если истории меньше N,
// используется самая старая доступная свеча.
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
