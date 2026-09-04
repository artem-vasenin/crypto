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

func sumVolumes(c []bybit.Candle, n int) float64 {
	if len(c) < n {
		n = len(c)
	}
	var s float64
	for _, v := range c[len(c)-n:] {
		s += v.Volume
	}
	return s
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
func change(c []bybit.Candle, n int) float64 {
	v := closes(c)
	if len(v) <= n || v[len(v)-1-n] == 0 {
		return 0
	}
	return indicators.PercentChange(v, n)
}
func changeByValues(v []float64, n int) float64 {
	if len(v) <= n || v[len(v)-1-n] == 0 {
		return 0
	}
	return (v[len(v)-1]/v[len(v)-1-n] - 1) * 100
}
func pct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a/b - 1) * 100
}
func ratioPct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b * 100
}

func takeLastCandles(c []bybit.Candle, n int) []bybit.Candle {
	if len(c) <= n {
		return c
	}
	return c[len(c)-n:]
}

func Build(ctx context.Context, api MarketAPI, symbol string, days int) (Report, error) {
	if days < 1 {
		days = 1
	}
	if days > 30 {
		days = 30 // расширяем лимит до 30 дней
	}
	ticker, err := api.Ticker(ctx, symbol)
	if err != nil {
		return Report{}, err
	}
	if ticker.LastPrice <= 0 {
		return Report{}, fmt.Errorf("получена некорректная цена %s", symbol)
	}

	c15, err := api.Klines(ctx, symbol, "15", 200)
	if err != nil {
		return Report{}, err
	}
	c1h, err := api.Klines(ctx, symbol, "60", 200)
	if err != nil {
		return Report{}, err
	}
	// Запрашиваем максимум 200 свечей 4h (33 дня)
	// Но для 30 дней нам нужно 180 свечей (30*6)
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

	// --- Базовые индикаторы (как было) ---
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

	// --- 14-дневные индикаторы (84 свечи 4h = 14 дней) ---
	c4h14d := takeLastCandles(c4h, 84)
	p4_14d := closes(c4h14d)
	e4_14d_20 := indicators.EMA(p4_14d, 20)
	e4_14d_50 := indicators.EMA(p4_14d, 50)
	e4_14d_200 := indicators.EMA(p4_14d, 200)
	atr4_14d := indicators.ATR(toIndicator(c4h14d), 84)
	rsi4_14d := indicators.RSI(p4_14d, 84)
	vol14d := sumVolumes(c4h, 84)
	vol7d := sumVolumes(c4h, 42)
	volRatio14d := 0.0
	if vol7d > 0 {
		volRatio14d = vol14d / vol7d
	}

	// --- 30-дневные индикаторы (180 свечей 4h = 30 дней) ---
	c4h30d := takeLastCandles(c4h, 180)
	p4_30d := closes(c4h30d)
	e4_30d_20 := indicators.EMA(p4_30d, 20)
	e4_30d_50 := indicators.EMA(p4_30d, 50)
	e4_30d_200 := indicators.EMA(p4_30d, 200)
	atr4_30d := indicators.ATR(toIndicator(c4h30d), 180)
	rsi4_30d := indicators.RSI(p4_30d, 180)

	m := Market{
		Price:        ticker.LastPrice,
		Change24hPct: ticker.Price24hPct,
		Change3dPct:  change(c1h, 72),
		Change7dPct:  change(c4h, 42),
		Turnover24h:  ticker.Turnover24h,
		Volume24h:    ticker.Volume24h,
		SpreadPct:    pct(ticker.AskPrice, ticker.BidPrice),
	}

	ind := Indicators{
		RSI15m:         indicators.RSI(p15, 14),
		RSI1h:          indicators.RSI(p1, 14),
		RSI4h:          indicators.RSI(p4, 14),
		RSI4h14d:       rsi4_14d,
		RSI4h30d:       rsi4_30d,
		ATR15m:         atr15,
		ATR1h:          atr1,
		ATR4h:          atr4,
		ATR4h14d:       atr4_14d,
		ATR4h30d:       atr4_30d,
		ATR1hPct:       ratioPct(atr1, ticker.LastPrice),
		ATR4hPct:       ratioPct(atr4, ticker.LastPrice),
		ATR4h14dPct:    ratioPct(atr4_14d, ticker.LastPrice),
		ATR4h30dPct:    ratioPct(atr4_30d, ticker.LastPrice),
		VolumeRatio1h:  indicators.VolumeRatio(vols(c1h), 6, 24),
		VolumeRatio14d: volRatio14d,
	}

	trend := Trend{
		EMA20_15m:              last(e15),
		EMA50_15m:              last(e15_50),
		EMA200_15m:             last(e15_200),
		EMA20_1h:               last(e1),
		EMA50_1h:               last(e1_50),
		EMA200_1h:              last(e1_200),
		EMA20_4h:               last(e4),
		EMA50_4h:               last(e4_50),
		EMA200_4h:              last(e4_200),
		EMA20_4h_14d:           last(e4_14d_20),
		EMA50_4h_14d:           last(e4_14d_50),
		EMA200_4h_14d:          last(e4_14d_200),
		EMA20_4h_30d:           last(e4_30d_20),
		EMA50_4h_30d:           last(e4_30d_50),
		EMA200_4h_30d:          last(e4_30d_200),
		PriceVsEMA20_1hPct:     pct(ticker.LastPrice, last(e1)),
		PriceVsEMA50_1hPct:     pct(ticker.LastPrice, last(e1_50)),
		PriceVsEMA200_1hPct:    pct(ticker.LastPrice, last(e1_200)),
		PriceVsEMA20_4h_14dPct: pct(ticker.LastPrice, last(e4_14d_20)),
		PriceVsEMA50_4h_14dPct: pct(ticker.LastPrice, last(e4_14d_50)),
	}

	mom := Momentum{
		Change1hPct:  change(c1h, 1),
		Change4hPct:  change(c1h, 4),
		Change12hPct: change(c1h, 12),
		Change24hPct: change(c1h, 24),
		Change7dPct:  change(c4h, 42),
		Change14dPct: changeByValues(p4_14d, len(p4_14d)-1),
		Change30dPct: changeByValues(p4_30d, len(p4_30d)-1),
		ROC1hPct:     change(c1h, 1),
		ROC4hPct:     change(c1h, 4),
		ROC14dPct:    changeByValues(p4_14d, len(p4_14d)-1),
	}

	vol := Volume{
		Volume5m:  sumRecent(c1m, 5),
		Volume15m: sumRecent(c1m, 15),
		Volume1h:  sumRecent(c1m, 60),
		Ratio5m:   volumeRatioRecent(c1m, 5, 60),
		Ratio15m:  volumeRatioRecent(c1m, 15, 60),
		Ratio1h:   volumeRatioRecent(c1m, 60, 240),
	}

	structure := buildStructure(c1h)
	levels := buildLevels(c1h, c4h, atr1, atr4, ticker.LastPrice)

	der := Derivatives{
		FundingRate:  ticker.FundingRate,
		OpenInterest: ticker.OpenInterest,
	}
	if len(funding) > 0 {
		der.FundingAvg = avgFunding(funding)
		der.FundingAvg24h = avgFundingSince(funding, time.Now().UTC().Add(-24*time.Hour))
	}
	if len(oi) > 1 {
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
	order := OrderBook{
		BidNotional:  ob.BidNotional,
		AskNotional:  ob.AskNotional,
		ImbalancePct: ob.ImbalancePct,
		BidAskRatio:  ob.Ratio,
		SpreadPct:    m.SpreadPct,
		Levels:       ob.Levels,
	}
	btc := buildBTCContext(c1m, btc1m, btcTicker, c1h, symbol == "BTCUSDT")
	strategies := scoreStrategies(m, ind, trend, mom, structure, levels, der, order, btc)

	notes := []string{}
	requested1m := min(1000, days*24*60)
	if len(c1m) < requested1m {
		notes = append(notes, fmt.Sprintf("Доступно %d из %d запрошенных 1m свечей; lead-lag рассчитан по доступной истории.", len(c1m), requested1m))
	}
	if days > 7 {
		notes = append(notes, "Для периодов >7 дней 1m свечи ограничены 1000 (7 дней), но 14- и 30-дневные метрики рассчитаны по 4h свечам.")
	}
	if days > 1 {
		notes = append(notes, "В текущей версии один запрос Bybit ограничен 1000 свечами; параметр days ограничивает запрос максимум 30 днями, но не выполняет постраничную загрузку сверх лимита API.")
	}
	if symbol == "BTCUSDT" {
		notes = append(notes, "Для BTCUSDT BTC lead-lag сравнивает инструмент с самим BTC и поэтому не является независимым сигналом.")
	}
	return Report{
		GeneratedAt: time.Now().UTC(),
		Exchange:    "Bybit",
		Category:    "linear",
		Symbol:      symbol,
		Purpose:     "Глубокий снимок одной монеты для последующего AI-анализа Long/Short и оценки контекста BTC.",
		DataQuality: DataQuality{OneMinuteCandles: len(c1m), Notes: notes},
		Market:      m,
		Indicators:  ind,
		Trend:       trend,
		Momentum:    mom,
		Volume:      vol,
		Structure:   structure,
		Levels:      levels,
		Derivatives: der,
		OrderBook:   order,
		BTCContext:  btc,
		Strategies:  strategies,
		AIInstructions: AIInstructions{
			Task: "Проанализируй, есть ли сейчас статистически и технически обоснованный сценарий LONG или SHORT по указанной монете.",
			Rules: []string{
				"Не принимать score как готовый торговый сигнал — проверять исходные метрики.",
				"Учитывать одновременно 15m/1h/4h, momentum, RSI, ATR, объём, funding, OI, long/short ratio, стакан и уровни.",
				"Отдельно проверить BTC-контекст и lead-lag: совпадает ли текущее движение монеты с BTC и есть ли исторический лаг.",
				"Обратить внимание на 14- и 30-дневные метрики (rsi_4h_14d, atr_4h_14d, change_14d_pct, range_14d_*): они показывают глобальный тренд и перекупленность.",
				"Если RSI_4h_14d > 80 и RSI_4h < 60 — это сигнал к скорой коррекции.",
				"Если RSI_4h_14d < 30 и RSI_4h > 50 — сигнал к скорому импульсу вверх.",
				"Если change_14d_pct > 40% — монета в параболическом ралли, шорт рискован.",
				"Если change_14d_pct < -20% — монета в глубокой коррекции, лонг рискован.",
				"Не придумывать данные, которых нет в JSON.",
				"Если преимущества LONG/SHORT недостаточно — прямо написать WAIT.",
			},
			RequestedOutput: []string{
				"Итог: LONG / SHORT / WAIT.",
				"Уровень уверенности 0-100.",
				"Точка или зона входа.",
				"SL и 1-3 цели TP с объяснением.",
				"Главный сценарий и сценарий отмены.",
				"Какие метрики сильнее всего подтверждают решение и какие ему противоречат.",
			},
		},
	}, nil
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

func buildLevels(c1h, c4h []bybit.Candle, atr1h, atr4h, price float64) Levels {
	if len(c1h) == 0 {
		return Levels{}
	}

	// --- 7-дневные уровни (как было) ---
	window := 120
	if len(c1h) < window {
		window = len(c1h)
	}
	cs := c1h[len(c1h)-window:]
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

	// --- 14-дневные уровни ---
	levels14d := buildLevelsFromCandles(c4h, 84, price)
	// --- 30-дневные уровни ---
	levels30d := buildLevelsFromCandles(c4h, 180, price)

	return Levels{
		Resistance:        tail(res, 5),
		Support:           tail(sup, 5),
		NearestResistance: nr,
		NearestSupport:    ns,
		RangeWidthPct:     width,
		RangePositionPct:  pos,
		RangeToATR1h: func() float64 {
			if atr1h == 0 {
				return 0
			}
			return (hi - lo) / atr1h
		}(),
		PriceDiscovery:      priceDiscovery,
		RecentRangeHigh:     hi,
		RecentRangeLow:      lo,
		Range14dHigh:        levels14d.High,
		Range14dLow:         levels14d.Low,
		Range14dWidthPct:    levels14d.WidthPct,
		Range14dPositionPct: levels14d.PositionPct,
		Range14dToATR4h:     levels14d.ToATR,
		Range30dHigh:        levels30d.High,
		Range30dLow:         levels30d.Low,
		Range30dWidthPct:    levels30d.WidthPct,
		Range30dPositionPct: levels30d.PositionPct,
	}
}

type rangeLevels struct {
	High, Low, WidthPct, PositionPct, ToATR float64
}

func buildLevelsFromCandles(c []bybit.Candle, window int, price float64) rangeLevels {
	if len(c) < 2 {
		return rangeLevels{}
	}
	if window > len(c) {
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
	width := 0.0
	if lo > 0 {
		width = (hi/lo - 1) * 100
	}
	pos := 0.0
	if hi > lo {
		pos = (price - lo) / (hi - lo) * 100
	}
	atr := indicators.ATR(toIndicator(cs), 14)
	toATR := 0.0
	if atr > 0 {
		toATR = (hi - lo) / atr
	}
	return rangeLevels{High: hi, Low: lo, WidthPct: width, PositionPct: pos, ToATR: toATR}
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

	// --- Существующие проверки ---
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

	// --- НОВЫЕ проверки на 14-дневные метрики ---

	// 1. Глобальная перекупленность → сигнал к коррекции (шорт)
	if ind.RSI4h14d > 80 && ind.RSI4h < 70 {
		short += 15 // RSI на 14д перекуплен, а на 7д уже остывает — дивергенция
	}
	if ind.RSI4h14d > 80 && ind.RSI4h > 80 {
		short += 5 // экстремальная перекупленность на всех ТФ
	}

	// 2. Глобальная перепроданность → сигнал к импульсу (лонг)
	if ind.RSI4h14d < 30 && ind.RSI4h > 50 {
		long += 15 // RSI на 14д перепродан, на 7д уже разворачивается
	}
	if ind.RSI4h14d < 30 && ind.RSI4h < 30 {
		long += 5 // экстремальная перепроданность
	}

	// 3. Сильный долгосрочный тренд
	if mom.Change14dPct > 20 && mom.Change4hPct > 0 {
		long += 10 // сильный бычий импульс за 14 дней
	}
	if mom.Change14dPct < -20 && mom.Change4hPct < 0 {
		short += 10 // сильный медвежий импульс за 14 дней
	}

	// 4. Тренд ускоряется (EMA20_4h_14d vs EMA50_4h_14d)
	if tr.EMA20_4h_14d > tr.EMA50_4h_14d && tr.EMA20_4h > tr.EMA50_4h {
		long += 10 // тренд ускоряется на всех горизонтах
	}
	if tr.EMA20_4h_14d < tr.EMA50_4h_14d && tr.EMA20_4h < tr.EMA50_4h {
		short += 10 // тренд ускоряется вниз
	}

	// 5. Глобальный диапазон — позиция у верха/низа
	if l.Range14dPositionPct > 90 {
		short += 10 // цена у верха глобального диапазона
		sg += 15
	}
	if l.Range14dPositionPct < 10 {
		long += 10 // цена у низа глобального диапазона
		lg += 15
	}

	// 6. Сжатие/расширение волатильности
	if ind.ATR4h14d < ind.ATR4h*0.7 && ind.ATR4h14d > 0 {
		// волатильность сжимается — готовимся к пробою (grid-стратегии)
		lg += 10
		sg += 10
	}
	if ind.ATR4h14d > ind.ATR4h*1.5 {
		// волатильность расширяется — тренд набирает силу
		if mom.Change14dPct > 0 {
			long += 10
		} else {
			short += 10
		}
	}

	// 7. Пробой глобального хая с объёмом
	if m.Price > l.Range14dHigh && ind.VolumeRatio14d > 1.2 {
		long += 10 // мощный пробой с объёмом
	}
	if m.Price < l.Range14dLow && ind.VolumeRatio14d > 1.2 {
		short += 10 // мощный пробой вниз с объёмом
	}

	// 8. Расстояние до глобальных EMA
	if tr.PriceVsEMA20_4h_14dPct > 30 {
		short += 5 // слишком далеко от EMA20 — перегрет
	}
	if tr.PriceVsEMA20_4h_14dPct < -20 {
		long += 5 // слишком далеко вниз от EMA20
	}

	// 9. 14-дневный RSI дивергенция (цена растёт, RSI падает)
	// Это сложно сделать без исторических данных, но мы можем хотя бы сравнить
	// RSI4h14d и RSI4h: если RSI4h14d ниже RSI4h, а цена выше, чем 14 дней назад — дивергенция
	if mom.Change14dPct > 10 && ind.RSI4h14d < ind.RSI4h {
		short += 10 // потенциальная медвежья дивергенция
	}
	if mom.Change14dPct < -10 && ind.RSI4h14d > ind.RSI4h {
		long += 10 // потенциальная бычья дивергенция
	}

	return Strategies{
		Long:        mk(long, "directional trend + momentum + volume + derivatives + 14d context"),
		LongGrid:    mk(lg, "trend + volatility + support + grid range + 14d range"),
		NeutralGrid: mk(ng, "range + trend neutrality + volatility + price position + liquidity"),
		Short:       mk(short, "directional downtrend + momentum + volume + derivatives + 14d context"),
		ShortGrid:   mk(sg, "impulse + volatility + structure + resistance + derivatives + 14d range"),
	}
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
