package analysis

import (
	"crypto-coin-analyzer/internal/bybit"
	"testing"
	"time"
)

// TestStructureCoordinates проверяет принципиальное свойство v3.1: структура должна сопровождаться
// реальными координатами swing points, а не быть непрозрачным текстовым label.
func TestStructureCoordinates(t *testing.T) {
	base := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	highs := []float64{10, 11, 14, 11, 10, 12, 15, 12, 11, 13, 16, 13, 12}
	lows := []float64{9, 10, 11, 8, 9, 10, 12, 9, 10, 11, 13, 10, 11}
	c := make([]bybit.Candle, len(highs))
	for i := range c {
		c[i] = bybit.Candle{Time: base.Add(time.Duration(i) * time.Hour), High: highs[i], Low: lows[i], Open: (highs[i] + lows[i]) / 2, Close: (highs[i] + lows[i]) / 2}
	}
	s := structure(c, 1)
	if len(s.LastSwings) < 4 {
		t.Fatalf("ожидались swing coordinates, получено %d", len(s.LastSwings))
	}
	if s.SwingHighSequence == "unknown" || s.SwingLowSequence == "unknown" {
		t.Fatalf("структура не определена: %+v", s)
	}
}

// TestDriftingRangeIsNotGridCandidate фиксирует защиту от старой ошибки: moving range не должен
// становиться хорошим GRID только из-за большого числа локальных колебаний.
func TestDriftingRangeIsNotGridCandidate(t *testing.T) {
	base := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	c := make([]bybit.Candle, 192)
	for i := range c {
		center := 100 + float64(i)*0.03
		wave := float64((i%8)-4) * 0.08
		c[i] = bybit.Candle{Time: base.Add(time.Duration(i) * 15 * time.Minute), Open: center + wave, Close: center - wave, High: center + 0.6, Low: center - 0.6, Volume: 100}
	}
	r := analyzeRange(c)
	if r.Stationarity != "drifting" {
		t.Fatalf("ожидался drifting range, получено %s drift=%.2f", r.Stationarity, r.RollingMidDriftPct)
	}
}

// TestFlowWindowRequiresFullCoverage не позволяет выдавать 5m/15m taker delta,
// если endpoint recent-trade фактически вернул только короткий snapshot.
func TestFlowWindowRequiresFullCoverage(t *testing.T) {
	base := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	tr := []bybit.Trade{
		{Time: base, Price: 100, Size: 1, Side: "Buy"},
		{Time: base.Add(30 * time.Second), Price: 101, Size: 1, Side: "Sell"},
	}
	one := flow(tr, time.Minute)
	if one.Available || one.Status != "insufficient_data" {
		t.Fatalf("30 секунд не должны изображать полное 1m окно: %+v", one)
	}
	tr = append(tr, bybit.Trade{Time: base.Add(6 * time.Minute), Price: 102, Size: 2, Side: "Buy"})
	five := flow(tr, 5*time.Minute)
	if !five.Available || five.Status != "complete_window" {
		t.Fatalf("ожидалось полноценное 5m окно: %+v", five)
	}
}

// TestOutsideBarDoesNotDefineStructure проверяет, что свеча, являющаяся одновременно pivot high и low,
// остаётся видимой в evidence, но не искажает последовательности HH/HL/LH/LL.
func TestOutsideBarDoesNotDefineStructure(t *testing.T) {
	base := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	c := []bybit.Candle{
		{Time: base, High: 10, Low: 9},
		{Time: base.Add(time.Hour), High: 12, Low: 7}, // outside pivot: и high, и low
		{Time: base.Add(2 * time.Hour), High: 10, Low: 9},
	}
	s := structure(c, 1)
	if len(s.LastSwings) != 2 || !s.LastSwings[0].Ambiguous || !s.LastSwings[1].Ambiguous {
		t.Fatalf("outside-bar должен быть явно маркирован двумя evidence points: %+v", s.LastSwings)
	}
	if s.SwingHighSequence != "unknown" || s.SwingLowSequence != "unknown" {
		t.Fatalf("один ambiguous outside-bar не должен формировать структуру: %+v", s)
	}
}
