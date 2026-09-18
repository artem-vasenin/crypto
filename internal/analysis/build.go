// Package analysis превращает сырые данные Bybit в универсальный evidence-first отчёт для GRID и directional анализа.
package analysis

import (
	"context"
	"crypto-coin-analyzer/internal/bybit"
	"crypto-coin-analyzer/internal/indicators"
	"fmt"
	"math"
	"time"
)

type MarketAPI interface {
	Ticker(context.Context, string) (bybit.Ticker, error)
	KlinesRange(context.Context, string, string, int) ([]bybit.Candle, error)
	MarkKlinesRange(context.Context, string, string, int) ([]bybit.PriceCandle, error)
	IndexKlinesRange(context.Context, string, string, int) ([]bybit.PriceCandle, error)
	Funding(context.Context, string, int) ([]bybit.Funding, error)
	OpenInterest(context.Context, string, string, int) ([]bybit.OpenInterest, error)
	LongShort(context.Context, string, string, int) ([]bybit.LongShort, error)
	OrderBook(context.Context, string, int) (bybit.OrderBook, error)
	RecentTrades(context.Context, string, int) ([]bybit.Trade, error)
}

func closes(c []bybit.Candle) []float64 {
	o := make([]float64, len(c))
	for i, x := range c {
		o[i] = x.Close
	}
	return o
}
func vols(c []bybit.Candle) []float64 {
	o := make([]float64, len(c))
	for i, x := range c {
		o[i] = x.Volume
	}
	return o
}
func ic(c []bybit.Candle) []indicators.Candle {
	o := make([]indicators.Candle, len(c))
	for i, x := range c {
		o[i] = indicators.Candle{Open: x.Open, High: x.High, Low: x.Low, Close: x.Close, Volume: x.Volume}
	}
	return o
}
func last(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}
func pct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a/b - 1) * 100
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func mean(v []float64) float64 { return indicators.Mean(v) }
func tail(v []float64, n int) []float64 {
	if len(v) <= n {
		return v
	}
	return v[len(v)-n:]
}
func tf(c []bybit.Candle, price float64, window int) Timeframe {
	p := closes(c)
	e20, e50, e200 := indicators.EMA(p, 20), indicators.EMA(p, 50), indicators.EMA(p, 200)
	atr := indicators.ATR(ic(c), 14)
	hi, lo := 0.0, math.MaxFloat64
	if window > len(c) {
		window = len(c)
	}
	for _, x := range c[len(c)-window:] {
		if x.High > hi {
			hi = x.High
		}
		if x.Low < lo {
			lo = x.Low
		}
	}
	pos := 0.0
	if hi > lo {
		pos = (price - lo) / (hi - lo) * 100
	}
	vr := 0.0
	v := vols(c)
	if len(v) >= 48 {
		vr = mean(tail(v, 6)) / mean(tail(v, 48))
	}
	return Timeframe{len(c), indicators.PercentChange(p, min(window-1, len(p)-1)), indicators.RSI(p, 14), atr, func() float64 {
		if price == 0 {
			return 0
		}
		return atr / price * 100
	}(), indicators.ADX(ic(c), 14), last(e20), last(e50), last(e200), indicators.EfficiencyRatio(p, min(window-1, 48)), indicators.RealizedVolPct(p, min(window-1, 48)), indicators.BollingerWidthPct(p, min(window, 48)), vr, hi, lo, pos}
}

// analyzeRange исследует последние 48 часов и отдельно измеряет дрейф rolling-range.
// Это защищает от ошибки, когда растущий/падающий канал ошибочно считается стационарным боковиком.
func analyzeRange(c []bybit.Candle) RangeAnalysis {
	n := min(len(c), 192)
	if n < 48 {
		return RangeAnalysis{}
	}
	x := c[len(c)-n:]
	hi, lo := 0.0, math.MaxFloat64
	for _, q := range x {
		if q.High > hi {
			hi = q.High
		}
		if q.Low < lo {
			lo = q.Low
		}
	}
	mid := (hi + lo) / 2
	width := hi - lo
	tol := width * .08
	ut, lt, mc := 0, 0, 0
	for i, q := range x {
		if q.High >= hi-tol {
			ut++
		}
		if q.Low <= lo+tol {
			lt++
		}
		if i > 0 && (x[i-1].Close-mid)*(q.Close-mid) < 0 {
			mc++
		}
	}
	pos := 0.0
	if width > 0 {
		pos = (x[len(x)-1].Close - lo) / width * 100
	}
	// Сравниваем границы первой и второй половины окна: это лучше отражает moving range, чем один global midpoint.
	bounds := func(z []bybit.Candle) (float64, float64) {
		h, l := 0.0, math.MaxFloat64
		for _, q := range z {
			if q.High > h {
				h = q.High
			}
			if q.Low < l {
				l = q.Low
			}
		}
		return h, l
	}
	h1, l1 := bounds(x[:n/2])
	h2, l2 := bounds(x[n/2:])
	m1, m2 := (h1+l1)/2, (h2+l2)/2
	w1, w2 := h1-l1, h2-l2
	drift := pct(m2, m1)
	wchg := 0.0
	if w1 > 0 {
		wchg = (w2/w1 - 1) * 100
	}
	slope := drift / (float64(n) * .25 / 2)
	station := "stationary"
	risks := []string{}
	ev := []string{}
	if math.Abs(drift) > 1.0 {
		station = "drifting"
		risks = append(risks, fmt.Sprintf("rolling midpoint сместился на %.2f%%", drift))
	} else {
		ev = append(ev, "rolling midpoint относительно стабилен")
	}
	if math.Abs(wchg) > 35 {
		risks = append(risks, fmt.Sprintf("ширина rolling range изменилась на %.1f%%", wchg))
	}
	bal := "balanced"
	if ut > lt*2 {
		bal = "upper-heavy"
	}
	if lt > ut*2 {
		bal = "lower-heavy"
	}
	if mc >= 3 {
		ev = append(ev, fmt.Sprintf("midpoint пересечён %d раз", mc))
	} else {
		risks = append(risks, "мало возвратов через midpoint")
	}
	if ut >= 3 && lt >= 3 {
		ev = append(ev, "обе границы тестировались неоднократно")
	} else {
		risks = append(risks, "нет достаточного числа тестов обеих границ")
	}
	return RangeAnalysis{48, hi, lo, mid, pct(hi, lo), pos, ut, lt, mc, 0, 0, 0, slope, drift, wchg, station, bal, ev, risks}
}

// swings ищет подтверждённые pivot high/low. Strength задаёт число свечей с каждой стороны экстремума.
func swings(c []bybit.Candle, strength, keep int) []SwingPoint {
	if len(c) < strength*2+1 {
		return nil
	}
	out := []SwingPoint{}
	for i := strength; i < len(c)-strength; i++ {
		hi, lo := true, true
		for j := i - strength; j <= i+strength; j++ {
			if j == i {
				continue
			}
			if c[j].High >= c[i].High {
				hi = false
			}
			if c[j].Low <= c[i].Low {
				lo = false
			}
		}
		if hi {
			out = append(out, SwingPoint{Time: c[i].Time, Type: "H", Price: c[i].High, Strength: strength, Ambiguous: hi && lo})
		}
		if lo {
			out = append(out, SwingPoint{Time: c[i].Time, Type: "L", Price: c[i].Low, Strength: strength, Ambiguous: hi && lo})
		}
	}
	if len(out) > keep {
		out = out[len(out)-keep:]
	}
	return out
}
func structure(c []bybit.Candle, strength int) StructureAnalysis {
	sw := swings(c, strength, 12)
	hs, ls := []SwingPoint{}, []SwingPoint{}
	ambiguousBars := 0
	seenAmbiguous := map[time.Time]bool{}
	for _, x := range sw {
		if x.Ambiguous {
			if !seenAmbiguous[x.Time] {
				ambiguousBars++
				seenAmbiguous[x.Time] = true
			}
			continue
		}
		if x.Type == "H" {
			hs = append(hs, x)
		} else {
			ls = append(ls, x)
		}
	}
	hseq, lseq := "unknown", "unknown"
	ev := []string{}
	if len(hs) >= 2 {
		if hs[len(hs)-1].Price > hs[len(hs)-2].Price {
			hseq = "HH"
		} else {
			hseq = "LH"
		}
		ev = append(ev, fmt.Sprintf("последние swing highs %.8g -> %.8g", hs[len(hs)-2].Price, hs[len(hs)-1].Price))
	}
	if len(ls) >= 2 {
		if ls[len(ls)-1].Price > ls[len(ls)-2].Price {
			lseq = "HL"
		} else {
			lseq = "LL"
		}
		ev = append(ev, fmt.Sprintf("последние swing lows %.8g -> %.8g", ls[len(ls)-2].Price, ls[len(ls)-1].Price))
	}
	label := hseq + "+" + lseq
	conflicts := []string{}
	if ambiguousBars > 0 {
		conflicts = append(conflicts, fmt.Sprintf("%d outside-bar swing(s) сохранены, но исключены из HH/HL/LH/LL", ambiguousBars))
	}
	return StructureAnalysis{label, hseq, lseq, sw, ev, conflicts}
}

// classifyRegime использует иерархию условий, а не сумму баллов. Сильные блокирующие признаки имеют приоритет.
func classifyRegime(t15, t1, t4 Timeframe, r RangeAnalysis) (MarketRegime, GridAnalysis) {
	ev := append([]string{}, r.Evidence...)
	risks := append([]string{}, r.Risks...)
	trend := "weak"
	if t1.ADX14 >= 25 {
		trend = "medium"
	}
	if t1.ADX14 >= 35 {
		trend = "strong"
	}
	vol := "normal"
	if t15.VolumeRatio > 1.5 || t1.ATRPct > t4.ATRPct {
		vol = "expansion"
	}
	if t15.VolumeRatio < .7 {
		vol = "compression"
	}
	br := "low"
	if t15.VolumeRatio > 1.5 || r.PositionPct < 8 || r.PositionPct > 92 {
		br = "medium"
	}
	if t15.VolumeRatio > 2.5 || t1.EfficiencyRatio > .6 {
		br = "high"
	}
	bias := "neutral"
	if t1.EMA20 > t1.EMA50 && t4.EMA20 > t4.EMA50 {
		bias = "long"
	}
	if t1.EMA20 < t1.EMA50 && t4.EMA20 < t4.EMA50 {
		bias = "short"
	}
	class := "RANGING_STATIONARY"
	if r.Stationarity == "drifting" {
		if r.RollingMidDriftPct > 0 {
			class = "RANGING_DRIFT_UP"
		} else if r.RollingMidDriftPct < 0 {
			class = "RANGING_DRIFT_DOWN"
		} else {
			class = "RANGING_DRIFT"
		}
	}
	if trend == "strong" && vol == "expansion" {
		class = "TRENDING_EXPANSION"
	} else if br == "high" {
		class = "BREAKOUT_RISK"
	} else if trend != "weak" {
		class = "TRENDING"
	} else if vol == "compression" {
		class = "COMPRESSION"
	}
	hard := []string{}
	if r.Stationarity == "drifting" {
		hard = append(hard, "range дрейфует")
	}
	if br == "high" {
		hard = append(hard, "высокий breakout risk")
	}
	if trend == "strong" && t1.EfficiencyRatio > .45 {
		hard = append(hard, "сильный эффективный тренд")
	}
	gridReg := "GRID_CANDIDATE"
	if len(hard) > 0 {
		gridReg = "NO_GRID_REGIME"
	} else if br == "medium" || trend == "medium" || len(r.Risks) > 0 {
		gridReg = "CAUTION"
	}
	side := func(dir string) SideAssessment {
		p, s, rf, hb := []string{}, []string{}, []string{}, append([]string{}, hard...)
		if dir == "long" {
			if r.PositionPct < 35 {
				p = append(p, "цена в нижней части range")
			} else {
				rf = append(rf, "цена не в нижней части range")
			}
			if bias == "long" {
				s = append(s, "EMA 1h/4h имеют bullish bias")
			}
		} else {
			if r.PositionPct > 65 {
				p = append(p, "цена в верхней части range")
			} else {
				rf = append(rf, "цена не в верхней части range")
			}
			if bias == "short" {
				s = append(s, "EMA 1h/4h имеют bearish bias")
			}
		}
		state := "OBSERVE"
		if len(hb) > 0 {
			state = "BLOCKED_BY_REGIME"
		} else if len(rf) > 0 {
			state = "CAUTION"
		}
		return SideAssessment{state, p, s, rf, hb, fmt.Sprintf("range_position=%.1f%%", r.PositionPct)}
	}
	rangeState := "uncertain"
	if r.Stationarity == "stationary" && r.UpperTouches >= 3 && r.LowerTouches >= 3 {
		rangeState = "established"
	}
	meanState := "weak"
	if r.MidCrosses >= 5 && r.Stationarity == "stationary" {
		meanState = "present"
	}
	return MarketRegime{class, trend, vol, br, bias, ev, risks}, GridAnalysis{gridReg, rangeState, meanState, br, side("long"), side("short"), ev, risks}
}

// analyzeDirectional не складывает индикаторы. Первичные факторы (4h/1h structure, trend, OI-price) отделены от вторичных.
func analyzeDirectional(t15, t1, t4 Timeframe, r RangeAnalysis, oi1, oi4, delta float64, s1, s4 StructureAnalysis) DirectionalAnalysis {
	bull, bear, conf := []string{}, []string{}, []string{}
	if s4.SwingHighSequence == "HH" && s4.SwingLowSequence == "HL" {
		bull = append(bull, "4h structure HH+HL")
	} else if s4.SwingHighSequence == "LH" && s4.SwingLowSequence == "LL" {
		bear = append(bear, "4h structure LH+LL")
	}
	if s1.SwingHighSequence == "HH" && s1.SwingLowSequence == "HL" {
		bull = append(bull, "1h structure HH+HL")
	} else if s1.SwingHighSequence == "LH" && s1.SwingLowSequence == "LL" {
		bear = append(bear, "1h structure LH+LL")
	}
	if t1.EMA20 > t1.EMA50 {
		bull = append(bull, "EMA20 > EMA50 1h")
	} else {
		bear = append(bear, "EMA20 < EMA50 1h")
	}
	if delta > 5 {
		bull = append(bull, "recent taker flow положительный")
	} else if delta < -5 {
		bear = append(bear, "recent taker flow отрицательный")
	}
	if t15.VolumeRatio > 1.8 {
		conf = append(conf, "volume expansion: возможен импульс или поздний вход")
	}
	if math.Abs(oi1) > 5 {
		conf = append(conf, "резкое изменение OI 1h")
	}
	mk := func(dir string) SideAssessment {
		primary, secondary, risks := []string{}, []string{}, append([]string{}, conf...)
		if dir == "long" {
			for _, x := range bull {
				if len(primary) < 3 {
					primary = append(primary, x)
				} else {
					secondary = append(secondary, x)
				}
			}
			if len(bear) >= 2 {
				risks = append(risks, "существенные bearish evidence присутствуют")
			}
		} else {
			for _, x := range bear {
				if len(primary) < 3 {
					primary = append(primary, x)
				} else {
					secondary = append(secondary, x)
				}
			}
			if len(bull) >= 2 {
				risks = append(risks, "существенные bullish evidence присутствуют")
			}
		}
		state := "MIXED"
		if len(primary) >= 2 && len(risks) == 0 {
			state = "SUPPORTED"
		}
		return SideAssessment{state, primary, secondary, risks, nil, fmt.Sprintf("range_position=%.1f%%; OI1h=%.2f%%; OI4h=%.2f%%", r.PositionPct, oi1, oi4)}
	}
	return DirectionalAnalysis{mk("long"), mk("short"), bull, bear, conf, []string{fmt.Sprintf("наблюдаемый low %.8g / high %.8g", r.Low, r.High)}, []string{fmt.Sprintf("наблюдаемые уровни %.8g–%.8g", r.Low, r.High)}}
}
func oiChange(v []bybit.OpenInterest, n int) float64 {
	if len(v) < 2 {
		return 0
	}
	if n >= len(v) {
		n = len(v) - 1
	}
	return pct(v[len(v)-1].Value, v[len(v)-1-n].Value)
}
func corrReturns(a, b []bybit.Candle, n int) float64 {
	n = min(n, min(len(a), len(b)))
	if n < 4 {
		return 0
	}
	aa, bb := closes(a[len(a)-n:]), closes(b[len(b)-n:])
	ra, rb := []float64{}, []float64{}
	for i := 1; i < n; i++ {
		if aa[i-1] != 0 && bb[i-1] != 0 {
			ra = append(ra, aa[i]/aa[i-1]-1)
			rb = append(rb, bb[i]/bb[i-1]-1)
		}
	}
	return indicators.Correlation(ra, rb)
}
func avgFunding(v []bybit.Funding, since time.Duration) float64 {
	cut := time.Now().UTC().Add(-since)
	z := []float64{}
	for _, x := range v {
		if x.Time.After(cut) {
			z = append(z, x.Rate)
		}
	}
	return mean(z)
}
func percentile(v []bybit.Funding, current float64) float64 {
	if len(v) == 0 {
		return 0
	}
	n := 0
	for _, x := range v {
		if x.Rate <= current {
			n++
		}
	}
	return float64(n) / float64(len(v)) * 100
}
func lsChange(v []bybit.LongShort, n int) float64 {
	if len(v) < 2 {
		return 0
	}
	if n >= len(v) {
		n = len(v) - 1
	}
	return (v[len(v)-1].LongRatio - v[len(v)-1-n].LongRatio) * 100
}
func priceOIState(price, oi float64) string {
	p := "price_flat"
	if price > .15 {
		p = "price_up"
	} else if price < -.15 {
		p = "price_down"
	}
	o := "oi_flat"
	if oi > .5 {
		o = "oi_up"
	} else if oi < -.5 {
		o = "oi_down"
	}
	return p + "+" + o
}
func flow(tr []bybit.Trade, window time.Duration) FlowWindow {
	// Bybit recent-trade для linear возвращает только последние сделки, а не гарантированное временное окно.
	// Поэтому 5m/15m delta нельзя выдавать, если фактическая выборка покрывает лишь десятки секунд.
	f := FlowWindow{Window: window.String(), RequiredCoverageSeconds: window.Seconds(), Status: "insufficient_data"}
	if len(tr) == 0 {
		return f
	}
	coverage := tr[len(tr)-1].Time.Sub(tr[0].Time).Seconds()
	f.ActualCoverageSeconds = coverage
	if coverage < window.Seconds() {
		return f
	}
	cut := tr[len(tr)-1].Time.Add(-window)
	b, sell, n := 0.0, 0.0, 0
	for _, x := range tr {
		if x.Time.Before(cut) {
			continue
		}
		n++
		v := x.Price * x.Size
		if x.Side == "Buy" {
			b += v
		} else {
			sell += v
		}
	}
	f.Available = true
	f.Status = "complete_window"
	f.Trades, f.BuyNotional, f.SellNotional = n, b, sell
	if b+sell > 0 {
		f.DeltaPct = (b - sell) / (b + sell) * 100
	}
	return f
}

func regimeHistory(c []bybit.Candle) []RegimeSnapshot {
	out := []RegimeSnapshot{}
	for _, ago := range []int{48, 36, 24, 12, 6, 0} {
		end := len(c) - ago*4
		if end < 200 {
			continue
		}
		z := c[:end]
		t := tf(z, z[len(z)-1].Close, min(192, len(z)))
		r := analyzeRange(z)
		cl := "RANGING"
		if t.ADX14 >= 35 && t.EfficiencyRatio > .45 {
			cl = "TRENDING"
		}
		if t.VolumeRatio > 2.5 {
			cl = "EXPANSION"
		}
		out = append(out, RegimeSnapshot{z[len(z)-1].Time, 48, cl, t.ADX14, t.EfficiencyRatio, t.ATRPct, t.VolumeRatio, r.SlopePctPerHour})
	}
	return out
}

// Build собирает полный raw evidence и нейтральные производные признаки по одному символу.
func Build(ctx context.Context, api MarketAPI, symbol string, request Request) (Report, error) {
	ticker, err := api.Ticker(ctx, symbol)
	if err != nil {
		return Report{}, err
	}
	if ticker.LastPrice <= 0 {
		return Report{}, fmt.Errorf("некорректная цена %s", symbol)
	}
	warnings := []string{}
	get := func(interval string, n int) []bybit.Candle {
		v, e := api.KlinesRange(ctx, symbol, interval, n)
		if e != nil {
			warnings = append(warnings, fmt.Sprintf("klines %s: %v", interval, e))
		}
		return v
	}
	c5, c15, c1, c4, cD := get("5", 864), get("15", 1344), get("60", 1440), get("240", 1080), get("D", 365)
	if len(c1) < 100 || len(c4) < 100 {
		return Report{}, fmt.Errorf("недостаточно основной истории для %s", symbol)
	}
	funding, e := api.Funding(ctx, symbol, 90)
	if e != nil {
		warnings = append(warnings, "funding: "+e.Error())
	}
	oi, e := api.OpenInterest(ctx, symbol, "5min", 288)
	if e != nil {
		warnings = append(warnings, "open interest: "+e.Error())
	}
	ls, e := api.LongShort(ctx, symbol, "5min", 288)
	if e != nil {
		warnings = append(warnings, "long/short: "+e.Error())
	}
	ob, e := api.OrderBook(ctx, symbol, 500)
	if e != nil {
		warnings = append(warnings, "orderbook: "+e.Error())
	}
	trades, e := api.RecentTrades(ctx, symbol, 1000)
	if e != nil {
		warnings = append(warnings, "recent trades: "+e.Error())
	}
	mark, e := api.MarkKlinesRange(ctx, symbol, "1", 60)
	if e != nil {
		warnings = append(warnings, "mark price: "+e.Error())
	}
	index, e := api.IndexKlinesRange(ctx, symbol, "1", 60)
	if e != nil {
		warnings = append(warnings, "index price: "+e.Error())
	}
	tf5, tf15, tf1, tf4, tfD := tf(c5, ticker.LastPrice, 288), tf(c15, ticker.LastPrice, 192), tf(c1, ticker.LastPrice, 168), tf(c4, ticker.LastPrice, 180), tf(cD, ticker.LastPrice, 90)
	rng := analyzeRange(c15)
	mr, grid := classifyRegime(tf15, tf1, tf4, rng)
	s5, s15, s1, s4 := structure(c5, 3), structure(c15, 3), structure(c1, 3), structure(c4, 2)
	buy, sell := 0.0, 0.0
	for _, x := range trades {
		v := x.Price * x.Size
		if x.Side == "Buy" {
			buy += v
		} else {
			sell += v
		}
	}
	delta := 0.0
	if buy+sell > 0 {
		delta = (buy - sell) / (buy + sell) * 100
	}
	var first, lastT time.Time
	coverage := 0.0
	if len(trades) > 0 {
		first = trades[0].Time
		lastT = trades[len(trades)-1].Time
		coverage = lastT.Sub(first).Seconds()
	}
	markP, indexP := 0.0, 0.0
	if len(mark) > 0 {
		markP = mark[len(mark)-1].Close
	}
	if len(index) > 0 {
		indexP = index[len(index)-1].Close
	}
	btcTicker, _ := api.Ticker(ctx, "BTCUSDT")
	btc1, _ := api.KlinesRange(ctx, "BTCUSDT", "60", 720)
	btc := BTCContext{Price: btcTicker.LastPrice}
	if len(btc1) > 25 {
		bp := closes(btc1)
		btc.Change1hPct = indicators.PercentChange(bp, 1)
		btc.Change4hPct = indicators.PercentChange(bp, 4)
		btc.Change24hPct = indicators.PercentChange(bp, 24)
		btc.Correlation1h30d = corrReturns(c1, btc1, 720)
		btc.RelativeStrength24hPct = ticker.Price24hPct - btc.Change24hPct
	}
	longNow := 0.0
	if len(ls) > 0 {
		longNow = ls[len(ls)-1].LongRatio
	}
	fundNow := ticker.FundingRate
	if len(funding) > 0 {
		fundNow = funding[len(funding)-1].Rate
	}
	oi15, oi1, oi4, oi24 := oiChange(oi, 3), oiChange(oi, 12), oiChange(oi, 48), oiChange(oi, 287)
	flow1m, flow5m, flow15m := flow(trades, time.Minute), flow(trades, 5*time.Minute), flow(trades, 15*time.Minute)
	// Directional evidence использует taker delta только при наличии полного 1m окна.
	// Snapshot короче минуты остаётся в JSON, но не получает права влиять на bullish/bearish evidence.
	directionalDelta := 0.0
	if flow1m.Available {
		directionalDelta = flow1m.DeltaPct
	}
	dir := analyzeDirectional(tf15, tf1, tf4, rng, oi1, oi4, directionalDelta, s1, s4)
	counts := map[string]int{"5m": len(c5), "15m": len(c15), "1h": len(c1), "4h": len(c4), "1d": len(cD), "oi_5m": len(oi), "long_short_5m": len(ls), "funding": len(funding), "recent_trades": len(trades), "mark_price_1m": len(mark), "index_price_1m": len(index)}
	depth := map[string]string{"5m": "~72 часа", "15m": "~14 дней", "1h": "~60 дней", "4h": "~180 дней", "1d": "~365 дней", "oi_5m": "~24 часа", "long_short_5m": "~24 часа", "mark/index_1m": "~60 минут"}
	quality := map[string]string{"ohlcv": "high", "derivatives": "high", "order_book": "snapshot_only", "recent_trades": "coverage_reported", "mark_index": "60m_1m"}
	ai := AIInstructions{"Проведи независимую интерпретацию raw evidence и derived metrics. Интегральные score намеренно отсутствуют: факторы имеют разный приоритет и не должны механически складываться.", []string{"1. Проверить data_quality и временное покрытие.", "2. Сначала определить regime и его переходы.", "3. Для GRID проверить stationarity/mean reversion/breakout risk, затем направление.", "4. Для directional приоритет: 4h/1h structure, impulse/pullback, volume и price-OI state; вторичные индикаторы использовать как контекст.", "5. Проверить derived metrics по raw evidence.", "6. Только внешний ИИ/человек формирует OPEN/WAIT/NO-GRID, entry, SL и targets."}, []string{"Нет score и скрытых весов: hard blocks, primary evidence, secondary evidence и risks разделены явно.", "Recent trades могут покрывать очень короткий период; taker_flow_windows.available=true только при полном временном покрытии окна.", "Order book — моментальный snapshot.", "Сильный trend/expansion плох для GRID, но может быть полезен directional."}, []string{"strategy", "status", "direction", "entry_condition", "grid_range или entry", "stop_loss", "targets", "аргументы и invalidation"}}
	return Report{"3.2", "3.2.0", timeNowUTC(), "Bybit", "linear / USDT perpetual", symbol, request, "Evidence-first deep analysis без интегральных score; финальную интерпретацию выполняет внешний ИИ/человек.", DataQuality{len(warnings) == 0, warnings, counts, depth, quality}, Market{ticker.LastPrice, ticker.Price24hPct, ticker.High24h, ticker.Low24h, ticker.Turnover24h, ticker.Volume24h, pct(ticker.AskPrice, ticker.BidPrice), ticker.FundingRate, ticker.OpenInterest, ticker.OpenInterestValue}, map[string]Timeframe{"5m": tf5, "15m": tf15, "1h": tf1, "4h": tf4, "1d": tfD}, map[string]StructureAnalysis{"5m": s5, "15m": s15, "1h": s1, "4h": s4}, mr, regimeHistory(c15), rng, grid, dir, Derivatives{funding, fundNow, avgFunding(funding, 24*time.Hour), avgFunding(funding, 7*24*time.Hour), percentile(funding, fundNow), oi, oi15, oi1, oi4, oi24, priceOIState(tf1.ChangePct, oi1), priceOIState(indicators.PercentChange(closes(c1), 4), oi4), ls, longNow, lsChange(ls, 12), lsChange(ls, 48), lsChange(ls, 287)}, Microstructure{ob, len(trades), first, lastT, coverage, buy, sell, delta, []FlowWindow{flow1m, flow5m, flow15m}, markP, indexP, pct(markP, indexP), pct(ticker.LastPrice, markP)}, btc, RawEvidence{c5, c15, c1, c4, cD, trades, mark, index}, ai}, nil
}

// timeNowUTC вынесена отдельно, чтобы generated_at всегда был явно UTC.
func timeNowUTC() time.Time { return time.Now().UTC() }
