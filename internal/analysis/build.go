package analysis

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"crypto-coin-analyzer/internal/bybit"
	"crypto-coin-analyzer/internal/indicators"
)

type MarketAPI interface {
	Ticker(context.Context, string) (bybit.Ticker, error)
	Klines(context.Context, string, string, int) ([]bybit.Candle, error)
	Funding(context.Context, string, int) ([]bybit.Funding, error)
	OpenInterest(context.Context, string, string, int) ([]bybit.OpenInterest, error)
	LongShort(context.Context, string, string, int) ([]bybit.LongShort, error)
	OrderBook(context.Context, string, int) (bybit.OrderBook, error)
}

func closes(c []bybit.Candle) []float64 {
	x := make([]float64, len(c))
	for i, v := range c {
		x[i] = v.Close
	}
	return x
}
func vols(c []bybit.Candle) []float64 {
	x := make([]float64, len(c))
	for i, v := range c {
		x[i] = v.Volume
	}
	return x
}
func change(c []bybit.Candle, n int) float64 { v := closes(c); return indicators.PercentChange(v, n) }
func pct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a/b - 1) * 100
}

// ratioPct возвращает отношение a к b в процентах.
// В отличие от pct это не процентное изменение, поэтому 644 / 81416 = 0.79%.
func ratioPct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b * 100
}

func Build(ctx context.Context, api MarketAPI, symbol string, days int) (Report, error) {
	if days < 1 {
		days = 1
	}
	if days > 7 {
		days = 7
	}
	ticker, err := api.Ticker(ctx, symbol)
	if err != nil {
		return Report{}, err
	}
	if ticker.LastPrice <= 0 {
		return Report{}, fmt.Errorf("получена некорректная цена %s", symbol)
	}

	// Параллелить запросы здесь можно, но последовательный вариант проще для публичного API
	// и снижает шанс словить rate-limit при запуске на слабом VPS.
	c15, err := api.Klines(ctx, symbol, "15", 200)
	if err != nil {
		return Report{}, err
	}
	c1h, err := api.Klines(ctx, symbol, "60", 200)
	if err != nil {
		return Report{}, err
	}
	c4h, err := api.Klines(ctx, symbol, "240", 200)
	if err != nil {
		return Report{}, err
	}
	c1m, err := api.Klines(ctx, symbol, "1", min(1000, days*24*60))
	if err != nil {
		return Report{}, err
	}
	btcTicker, err := api.Ticker(ctx, "BTCUSDT")
	if err != nil {
		return Report{}, err
	}
	btc1m, err := api.Klines(ctx, "BTCUSDT", "1", min(1000, days*24*60))
	if err != nil {
		return Report{}, err
	}
	funding, _ := api.Funding(ctx, symbol, 10)
	oi, _ := api.OpenInterest(ctx, symbol, "1h", 50)
	ls, _ := api.LongShort(ctx, symbol, "1h", 1)
	ob, _ := api.OrderBook(ctx, symbol, 200)

	p15, p1, p4 := closes(c15), closes(c1h), closes(c4h)
	e15 := indicators.EMA(p15, 20)
	e15_50 := indicators.EMA(p15, 50)
	e15_200 := indicators.EMA(p15, 200)
	e1 := indicators.EMA(p1, 20)
	e1_50 := indicators.EMA(p1, 50)
	e1_200 := indicators.EMA(p1, 200)
	e4 := indicators.EMA(p4, 20)
	e4_50 := indicators.EMA(p4, 50)
	e4_200 := indicators.EMA(p4, 200)
	atr15 := indicators.ATR(toIndicator(c15), 14)
	atr1 := indicators.ATR(toIndicator(c1h), 14)
	atr4 := indicators.ATR(toIndicator(c4h), 14)

	m := Market{Price: ticker.LastPrice, Change24hPct: ticker.Price24hPct, Change3dPct: change(c1h, 72), Change7dPct: change(c4h, 42), Turnover24h: ticker.Turnover24h, Volume24h: ticker.Volume24h, SpreadPct: pct(ticker.AskPrice, ticker.BidPrice)}
	ind := Indicators{RSI15m: indicators.RSI(p15, 14), RSI1h: indicators.RSI(p1, 14), RSI4h: indicators.RSI(p4, 14), ATR15m: atr15, ATR1h: atr1, ATR4h: atr4, ATR1hPct: ratioPct(atr1, ticker.LastPrice), ATR4hPct: ratioPct(atr4, ticker.LastPrice), VolumeRatio1h: indicators.VolumeRatio(vols(c1h), 6, 24)}
	trend := Trend{EMA20_15m: last(e15), EMA50_15m: last(e15_50), EMA200_15m: last(e15_200), EMA20_1h: last(e1), EMA50_1h: last(e1_50), EMA200_1h: last(e1_200), EMA20_4h: last(e4), EMA50_4h: last(e4_50), EMA200_4h: last(e4_200), PriceVsEMA20_1hPct: pct(ticker.LastPrice, last(e1)), PriceVsEMA50_1hPct: pct(ticker.LastPrice, last(e1_50)), PriceVsEMA200_1hPct: pct(ticker.LastPrice, last(e1_200))}
	mom := Momentum{Change1hPct: change(c1h, 1), Change4hPct: change(c1h, 4), Change12hPct: change(c1h, 12), Change24hPct: change(c1h, 24), ROC1hPct: change(c1h, 1), ROC4hPct: change(c1h, 4)}
	vol := Volume{Volume5m: sumRecent(c1m, 5), Volume15m: sumRecent(c1m, 15), Volume1h: sumRecent(c1m, 60), Ratio5m: volumeRatioRecent(c1m, 5, 60), Ratio15m: volumeRatioRecent(c1m, 15, 60), Ratio1h: volumeRatioRecent(c1m, 60, 240)}
	structure := buildStructure(c1h)
	levels := buildLevels(c1h, atr1, ticker.LastPrice)
	der := Derivatives{FundingRate: ticker.FundingRate, OpenInterest: ticker.OpenInterest}
	if len(funding) > 0 {
		der.FundingAvg = avgFunding(funding)
		der.FundingAvg24h = avgFundingSince(funding, time.Now().UTC().Add(-24*time.Hour))
	}
	if len(oi) > 1 {
		// Bybit может вернуть OI от новых записей к старым.
		// Сначала приводим ряд к old -> new, затем считаем изменение.
		sort.Slice(oi, func(i, j int) bool { return oi[i].Time.Before(oi[j].Time) })
		oldest, newest := oi[0].Value, oi[len(oi)-1].Value
		if oldest != 0 {
			der.OpenInterestChangePct = (newest/oldest - 1) * 100
		}
	}
	if len(ls) > 0 {
		der.LongRatio = ls[0].BuyRatio
		der.ShortRatio = ls[0].SellRatio
	}
	order := OrderBook{BidNotional: ob.BidNotional, AskNotional: ob.AskNotional, ImbalancePct: ob.ImbalancePct, BidAskRatio: ob.Ratio, SpreadPct: m.SpreadPct, Levels: ob.Levels}
	btc := buildBTCContext(c1m, btc1m, btcTicker, c1h, symbol == "BTCUSDT")
	strategies := scoreStrategies(m, ind, trend, mom, structure, levels, der, order, btc)

	notes := []string{}
	requested1m := min(1000, days*24*60)
	if len(c1m) < requested1m {
		notes = append(notes, fmt.Sprintf("Доступно %d из %d запрошенных 1m свечей; lead-lag рассчитан по доступной истории.", len(c1m), requested1m))
	}
	if days > 1 {
		notes = append(notes, "В текущей версии один запрос Bybit ограничен 1000 свечами; параметр days ограничивает запрос максимум 7 днями, но не выполняет постраничную загрузку сверх лимита API.")
	}
	if symbol == "BTCUSDT" {
		notes = append(notes, "Для BTCUSDT BTC lead-lag сравнивает инструмент с самим BTC и поэтому не является независимым сигналом.")
	}
	return Report{GeneratedAt: time.Now().UTC(), Exchange: "Bybit", Category: "linear", Symbol: symbol, Purpose: "Глубокий снимок одной монеты для последующего AI-анализа Long/Short и оценки контекста BTC.", DataQuality: DataQuality{OneMinuteCandles: len(c1m), Notes: notes}, Market: m, Indicators: ind, Trend: trend, Momentum: mom, Volume: vol, Structure: structure, Levels: levels, Derivatives: der, OrderBook: order, BTCContext: btc, Strategies: strategies, AIInstructions: AIInstructions{Task: "Проанализируй, есть ли сейчас статистически и технически обоснованный сценарий LONG или SHORT по указанной монете.", Rules: []string{"Не принимать score как готовый торговый сигнал — проверять исходные метрики.", "Учитывать одновременно 15m/1h/4h, momentum, RSI, ATR, объём, funding, OI, long/short ratio, стакан и уровни.", "Отдельно проверить BTC-контекст и lead-lag: совпадает ли текущее движение монеты с BTC и есть ли исторический лаг.", "Не придумывать данные, которых нет в JSON.", "Если преимущества LONG/SHORT недостаточно — прямо написать WAIT."}, RequestedOutput: []string{"Итог: LONG / SHORT / WAIT.", "Уровень уверенности 0-100.", "Точка или зона входа.", "SL и 1-3 цели TP с объяснением.", "Главный сценарий и сценарий отмены.", "Какие метрики сильнее всего подтверждают решение и какие ему противоречат."}}}, nil
}

func toIndicator(c []bybit.Candle) []indicators.Candle {
	o := make([]indicators.Candle, len(c))
	for i, v := range c {
		o[i] = indicators.Candle{Open: v.Open, High: v.High, Low: v.Low, Close: v.Close, Volume: v.Volume}
	}
	return o
}
func last(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func sumRecent(c []bybit.Candle, n int) float64 {
	if len(c) == 0 {
		return 0
	}
	if n > len(c) {
		n = len(c)
	}
	var s float64
	for _, v := range c[len(c)-n:] {
		s += v.Volume
	}
	return s
}
func volumeRatioRecent(c []bybit.Candle, n, long int) float64 {
	if len(c) < long {
		return 0
	}
	return indicators.Mean(volumes(c[len(c)-n:])) / indicators.Mean(volumes(c[len(c)-long:]))
}
func volumes(c []bybit.Candle) []float64 { return vols(c) }
func avgFundingSince(f []bybit.Funding, since time.Time) float64 {
	filtered := make([]bybit.Funding, 0, len(f))
	for _, x := range f {
		if !x.Time.Before(since) {
			filtered = append(filtered, x)
		}
	}
	if len(filtered) == 0 {
		return avgFunding(f)
	}
	return avgFunding(filtered)
}

func avgFunding(f []bybit.Funding) float64 {
	if len(f) == 0 {
		return 0
	}
	var s float64
	for _, x := range f {
		s += x.Rate
	}
	return s / float64(len(f))
}

func buildStructure(c []bybit.Candle) Structure {
	var highs, lows []Pivot
	if len(c) >= 5 {
		for i := 2; i < len(c)-2; i++ {
			if c[i].High > c[i-1].High && c[i].High > c[i-2].High && c[i].High > c[i+1].High && c[i].High > c[i+2].High {
				highs = append(highs, Pivot{Time: c[i].Time, Price: c[i].High})
			}
			if c[i].Low < c[i-1].Low && c[i].Low < c[i-2].Low && c[i].Low < c[i+1].Low && c[i].Low < c[i+2].Low {
				lows = append(lows, Pivot{Time: c[i].Time, Price: c[i].Low})
			}
		}
	}
	if len(highs) > 6 {
		highs = highs[len(highs)-6:]
	}
	if len(lows) > 6 {
		lows = lows[len(lows)-6:]
	}
	s := Structure{PivotHighs: highs, PivotLows: lows}
	if len(highs) >= 2 {
		s.PreviousHigh = highs[len(highs)-2].Price
		s.CurrentHigh = highs[len(highs)-1].Price
		if s.CurrentHigh > s.PreviousHigh {
			s.HighState = "HH"
		} else {
			s.HighState = "LH"
		}
	}
	if len(lows) >= 2 {
		s.PreviousLow = lows[len(lows)-2].Price
		s.CurrentLow = lows[len(lows)-1].Price
		if s.CurrentLow > s.PreviousLow {
			s.LowState = "HL"
		} else {
			s.LowState = "LL"
		}
	}
	return s
}
func buildLevels(c []bybit.Candle, atr, price float64) Levels {
	if len(c) == 0 {
		return Levels{}
	}
	window := 120
	if len(c) < window {
		window = len(c)
	}
	cs := c[len(c)-window:]
	hi, lo := cs[0].High, cs[0].Low
	for _, v := range cs {
		if v.High > hi {
			hi = v.High
		}
		if v.Low < lo {
			lo = v.Low
		}
	}
	res := []float64{}
	sup := []float64{}
	for i := 2; i < len(cs)-2; i++ {
		if cs[i].High > cs[i-1].High && cs[i].High > cs[i-2].High && cs[i].High > cs[i+1].High && cs[i].High > cs[i+2].High {
			if cs[i].High > price {
				res = append(res, cs[i].High)
			}
		}
		if cs[i].Low < cs[i-1].Low && cs[i].Low < cs[i-2].Low && cs[i].Low < cs[i+1].Low && cs[i].Low < cs[i+2].Low {
			if cs[i].Low < price {
				sup = append(sup, cs[i].Low)
			}
		}
	}
	sort.Float64s(res)
	sort.Sort(sort.Reverse(sort.Float64Slice(sup)))
	nr, ns := 0.0, 0.0
	if len(res) > 0 {
		nr = res[0]
	}
	priceDiscovery := len(res) == 0 && price >= hi
	if len(sup) > 0 {
		ns = sup[0]
	}
	width := pct(hi, lo)
	pos := 0.0
	if hi > lo {
		pos = (price - lo) / (hi - lo) * 100
	}
	return Levels{Resistance: tail(res, 5), Support: tail(sup, 5), NearestResistance: nr, NearestSupport: ns, RangeWidthPct: width, RangePositionPct: pos, RangeToATR1h: func() float64 {
		if atr == 0 {
			return 0
		}
		return (hi - lo) / atr
	}(), PriceDiscovery: priceDiscovery, RecentRangeHigh: hi, RecentRangeLow: lo}
}
func tail(v []float64, n int) []float64 {
	if len(v) <= n {
		return v
	}
	return v[len(v)-n:]
}

func buildBTCContext(c, btc []bybit.Candle, t bybit.Ticker, target1h []bybit.Candle, selfReference bool) BTCContext {
	ctx := BTCContext{Price: t.LastPrice, Change24hPct: t.Price24hPct, SelfReference: selfReference}
	btcHourly := btcTo1h(btc)
	ctx.Change1hPct = change(btcHourly, 1)
	ctx.Change4hPct = change(btcHourly, 4)

	if selfReference {
		ctx.Interpretation = "BTCUSDT выбран как анализируемый инструмент: BTC-контекст является самоссылочным и не используется как независимый lead-lag сигнал."
		return ctx
	}

	cr := alignReturns(c, btc)
	lags := []int{0, 1, 2, 3, 5, 10}
	vals := make([]float64, len(lags))
	for i, l := range lags {
		vals[i] = lagCorr(cr, l)
	}
	ctx.Corr1m0 = vals[0]
	ctx.Corr1m1 = vals[1]
	ctx.Corr1m2 = vals[2]
	ctx.Corr1m3 = vals[3]
	ctx.Corr1m5 = vals[4]
	ctx.Corr1m10 = vals[5]
	best := 0
	bestv := vals[0]
	for i := 1; i < len(vals); i++ {
		if vals[i] > bestv {
			bestv = vals[i]
			best = lags[i]
		}
	}
	ctx.BestLagMinutes = best
	ctx.BestLagCorrelation = bestv
	if len(target1h) > 4 {
		ctx.Relative1hPct = change(target1h, 1) - change(btcHourly, 1)
		ctx.Relative4hPct = change(target1h, 4) - change(btcHourly, 4)
	}
	ctx.Interpretation = fmt.Sprintf("Максимальная корреляция доходностей BTC→монета среди проверенных лагов: %d мин, corr=%.3f. Это статистический контекст, а не гарантия догоняющего движения.", best, bestv)
	return ctx
}

func btcTo1h(c []bybit.Candle) []bybit.Candle {
	if len(c) < 60 {
		return c
	}
	out := make([]bybit.Candle, 0, len(c)/60)
	for i := 0; i+59 < len(c); i += 60 {
		g := c[i : i+60]
		x := g[0]
		x.Close = g[len(g)-1].Close
		x.High = g[0].High
		x.Low = g[0].Low
		for _, v := range g {
			if v.High > x.High {
				x.High = v.High
			}
			if v.Low < x.Low {
				x.Low = v.Low
			}
		}
		out = append(out, x)
	}
	return out
}
func alignReturns(a, b []bybit.Candle) []float64 {
	if len(a) < 2 || len(b) < 2 {
		return nil
	}

	// Сопоставляем доходности по timestamp, а не по позиции в массиве.
	// Это защищает lead-lag от рассинхронизации рядов из-за пропущенных свечей.
	bReturns := make(map[time.Time]float64, len(b)-1)
	for i := 1; i < len(b); i++ {
		if b[i-1].Close != 0 {
			bReturns[b[i].Time] = b[i].Close/b[i-1].Close - 1
		}
	}

	aReturns := make([]float64, 0, len(a)-1)
	bAligned := make([]float64, 0, len(a)-1)
	for i := 1; i < len(a); i++ {
		if a[i-1].Close == 0 {
			continue
		}
		br, ok := bReturns[a[i].Time]
		if !ok {
			continue
		}
		aReturns = append(aReturns, a[i].Close/a[i-1].Close-1)
		bAligned = append(bAligned, br)
	}

	if len(aReturns) < 4 {
		return nil
	}
	out := make([]float64, 0, len(aReturns)*2)
	for i := range aReturns {
		out = append(out, aReturns[i], bAligned[i])
	}
	return out
}

func lagCorr(pairs []float64, lag int) float64 {
	if len(pairs) < 6 {
		return 0
	}
	a := make([]float64, 0, len(pairs)/2)
	b := make([]float64, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		a = append(a, pairs[i])
		b = append(b, pairs[i+1])
	}
	if lag >= len(a) {
		return 0
	}
	return indicators.Correlation(a[lag:], b[:len(b)-lag])
}

func scoreStrategies(m Market, ind Indicators, tr Trend, mom Momentum, s Structure, l Levels, d Derivatives, o OrderBook, btc BTCContext) Strategies {
	long := 0
	short := 0
	lg := 0
	sg := 0
	ng := 0
	if tr.EMA20_1h > tr.EMA50_1h {
		long += 15
	} else {
		short += 15
	}
	if tr.EMA20_4h > tr.EMA50_4h {
		long += 15
	} else {
		short += 15
	}
	if mom.Change4hPct > 1 {
		long += 10
	}
	if mom.Change4hPct < -1 {
		short += 10
	}
	if ind.RSI1h > 55 {
		long += 10
	}
	if ind.RSI1h < 45 {
		short += 10
	}
	if d.OpenInterestChangePct > 3 && mom.Change4hPct > 0 {
		long += 10
	}
	if d.OpenInterestChangePct > 3 && mom.Change4hPct < 0 {
		short += 10
	}
	if o.ImbalancePct > 5 {
		long += 5
	}
	if o.ImbalancePct < -5 {
		short += 5
	}
	if s.HighState == "HH" && s.LowState == "HL" {
		long += 10
	}
	if s.HighState == "LH" && s.LowState == "LL" {
		short += 10
	}
	if l.NearestSupport > 0 && pct(m.Price, l.NearestSupport) < 2 {
		lg += 15
	}
	if l.NearestResistance > 0 && pct(l.NearestResistance, m.Price) < 2 {
		sg += 15
	}
	if l.RangeToATR1h >= 2 && l.RangeWidthPct >= 2 {
		lg += 20
		sg += 20
	}
	if ind.ATR1hPct >= 0.5 {
		lg += 15
		sg += 15
	}
	if ind.VolumeRatio1h >= 1.1 {
		lg += 10
		sg += 10
	}
	if s.HighState == "LH" {
		sg += 10
	}
	if s.LowState == "HL" {
		lg += 10
	}
	if l.RangePositionPct < 35 {
		lg += 15
	}
	if l.RangePositionPct > 65 {
		sg += 15
	}
	if tr.EMA20_1h > tr.EMA50_1h && tr.EMA50_1h > tr.EMA200_1h {
		ng -= 15
	} else {
		ng += 5
	}
	if math.Abs(mom.Change24hPct) < 5 {
		ng += 15
	}
	if l.RangeWidthPct >= 2 && l.RangeWidthPct <= 15 {
		ng += 25
	}
	if ind.RSI1h >= 40 && ind.RSI1h <= 60 {
		ng += 15
	}
	if math.Abs(btc.Relative1hPct) > 1.5 {
		if btc.Relative1hPct > 0 {
			long += 5
		} else {
			short += 5
		}
	}
	return Strategies{Long: mk(long, "directional trend + momentum + volume + derivatives"), LongGrid: mk(lg, "trend + volatility + support + grid range"), NeutralGrid: mk(ng, "range + trend neutrality + volatility + price position + liquidity"), Short: mk(short, "directional downtrend + momentum + volume + derivatives"), ShortGrid: mk(sg, "impulse + volatility + structure + resistance + derivatives")}
}
func mk(score int, reason string) Strategy {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	status := "avoid"
	if score >= 75 {
		status = "consider"
	} else if score >= 55 {
		status = "watch"
	} else if score >= 35 {
		status = "risky"
	}
	return Strategy{Score: score, Status: status, Reason: reason}
}
