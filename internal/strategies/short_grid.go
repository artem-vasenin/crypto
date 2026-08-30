package strategies

import (
	"time"
	"universal-bybit-screener/models"
)

// ShortGrid ищет импульс вверх, который начал замедляться
// возле сопротивления и имеет признаки локального разворота.
//
// В отличие от обычного short, short-grid не должен просто искать
// падающие монеты. Нам нужен диапазон, в котором цена может ходить
// между сопротивлением и поддержкой.
//
// Поэтому стратегия сначала применяет жёсткие фильтры (hard gates),
// а уже после этого рассчитывает score.
type ShortGrid struct{}

func (ShortGrid) Name() string {
	return "short-grid"
}

func (ShortGrid) Evaluate(
	m models.MarketData,
	i models.Indicators,
	s map[string]models.Structure,
	l models.Levels,
) models.StrategyResult {

	st1h, ok1h := s["1h"]
	st15m, ok15m := s["15m"]
	st4h, ok4h := s["4h"]

	// ------------------------------------------------------------
	// HARD GATES
	// ------------------------------------------------------------

	// Для grid необходим полноценный диапазон.
	if l.NearestResistance <= 0 || l.NearestSupport <= 0 {
		return rejectShortGrid("нет полноценного диапазона support/resistance")
	}

	// Для short-grid обязательно нужен LH на 1h.
	if !ok1h || st1h.HighState != "LH" {
		return rejectShortGrid("на 1h нет подтверждённого LH")
	}

	// Явный бычий тренд на 4h запрещаем.
	if ok4h && st4h.HighState == "HH" && st4h.LowState == "HL" {
		return rejectShortGrid("на 4h сохраняется сильная бычья структура HH+HL")
	}

	// ------------------------------------------------------------
	// Свежий импульс
	// ------------------------------------------------------------

	// Для short-grid нам нужен недавний рост.
	//
	// 24h берём непосредственно из Ticker.
	// 3d рассчитываем по часовым свечам, потому что отдельного
	// поля Price3dPcnt в MarketData.Ticker нет.
	change3dPct := change3d(m.Candles1h)

	if m.Ticker.Price24hPcnt < 5 && change3dPct < 5 {
		return rejectShortGrid("нет достаточно свежего восходящего импульса")
	}

	// ------------------------------------------------------------
	// Сопротивление
	// ------------------------------------------------------------

	if m.Ticker.LastPrice <= 0 {
		return rejectShortGrid("некорректная текущая цена")
	}

	resistanceDistancePct :=
		(l.NearestResistance - m.Ticker.LastPrice) /
			m.Ticker.LastPrice * 100

	// Цена не должна уже находиться выше сопротивления.
	if resistanceDistancePct < 0 {
		return rejectShortGrid("цена уже выше ближайшего сопротивления")
	}

	// Слишком далёкое сопротивление нам не интересно.
	if resistanceDistancePct > 6 {
		return rejectShortGrid("сопротивление слишком далеко")
	}

	// ------------------------------------------------------------
	// Положение внутри диапазона
	// ------------------------------------------------------------

	// Short-grid открываем в верхней части диапазона.
	if l.RangePositionPct < 40 {
		return rejectShortGrid("цена находится слишком близко к поддержке")
	}

	// ------------------------------------------------------------
	// Размер диапазона
	// ------------------------------------------------------------

	// Диапазон должен быть минимум примерно 2 ATR.
	if l.RangeToATR1h < 2.0 {
		return rejectShortGrid("диапазон слишком узкий относительно ATR")
	}

	// Дополнительный абсолютный фильтр.
	if l.RangeWidthPct < 4 {
		return rejectShortGrid("абсолютная ширина диапазона слишком мала")
	}

	// ------------------------------------------------------------
	// Локальное ослабление
	// ------------------------------------------------------------

	if ok15m {
		if st15m.HighState != "LH" && st15m.LowState != "LL" {
			return rejectShortGrid("на 15m нет признаков локального ослабления")
		}
	}

	// ------------------------------------------------------------
	// SCORE
	// ------------------------------------------------------------

	score := 0.0

	// Свежий импульс 24h.
	if m.Ticker.Price24hPcnt >= 20 {
		score += 10
	} else if m.Ticker.Price24hPcnt >= 10 {
		score += 7
	} else if m.Ticker.Price24hPcnt >= 5 {
		score += 4
	}

	// Импульс за 3 дня.
	if change3dPct >= 20 {
		score += 7
	} else if change3dPct >= 10 {
		score += 5
	} else if change3dPct >= 5 {
		score += 3
	}

	// ------------------------------------------------------------
	// Волатильность
	// ------------------------------------------------------------

	if i.ATR1hPct >= 5 {
		score += 10
	} else if i.ATR1hPct >= 3 {
		score += 8
	} else if i.ATR1hPct >= 2 {
		score += 5
	} else if i.ATR1hPct >= 1.5 {
		score += 3
	}

	// ------------------------------------------------------------
	// Структура
	// ------------------------------------------------------------

	if st1h.HighState == "LH" {
		score += 12
	}

	if st1h.LowState == "LL" {
		score += 5
	} else if st1h.LowState == "HL" {
		score += 2
	}

	if ok15m {
		if st15m.HighState == "LH" {
			score += 7
		}

		if st15m.LowState == "LL" {
			score += 5
		}

		if st15m.HighState == "HH" && st15m.LowState == "HL" {
			score -= 8
		}
	}

	// ------------------------------------------------------------
	// Сопротивление
	// ------------------------------------------------------------

	switch {
	case resistanceDistancePct <= 2:
		score += 12
	case resistanceDistancePct <= 4:
		score += 9
	case resistanceDistancePct <= 6:
		score += 5
	}

	// ------------------------------------------------------------
	// Положение внутри диапазона
	// ------------------------------------------------------------

	switch {
	case l.RangePositionPct >= 70:
		score += 10
	case l.RangePositionPct >= 55:
		score += 7
	case l.RangePositionPct >= 40:
		score += 4
	}

	// ------------------------------------------------------------
	// Range / ATR
	// ------------------------------------------------------------

	switch {
	case l.RangeToATR1h >= 3:
		score += 8
	case l.RangeToATR1h >= 2.5:
		score += 6
	case l.RangeToATR1h >= 2:
		score += 4
	}

	// ------------------------------------------------------------
	// Volume
	// ------------------------------------------------------------

	if i.VolumeRatio1h >= 2 {
		score += 6
	} else if i.VolumeRatio1h >= 1.5 {
		score += 4
	} else if i.VolumeRatio1h >= 1.2 {
		score += 2
	}

	if i.VolumeTrend1h >= 2 {
		score += 5
	} else if i.VolumeTrend1h >= 1.2 {
		score += 3
	}

	// ------------------------------------------------------------
	// Funding
	// ------------------------------------------------------------

	if m.Ticker.FundingRate > 0.0001 {
		score += 5
	} else if m.Ticker.FundingRate > 0 {
		score += 2
	}

	if len(m.Funding) > 0 && m.Funding[0].Rate > 0 {
		score += 2
	}

	// ------------------------------------------------------------
	// Open Interest
	// ------------------------------------------------------------

	if len(m.OpenInterest) >= 2 {
		first := m.OpenInterest[0].OpenInterest
		last := m.OpenInterest[len(m.OpenInterest)-1].OpenInterest

		if first > 0 {
			oiChangePct := (last - first) / first * 100

			if oiChangePct <= -10 {
				score += 7
			} else if oiChangePct <= -5 {
				score += 5
			} else if oiChangePct < 0 {
				score += 2
			} else if oiChangePct >= 15 {
				score -= 5
			}
		}
	}

	// ------------------------------------------------------------
	// Order Book
	// ------------------------------------------------------------

	if m.OrderBook.ImbalancePct <= -15 {
		score += 6
	} else if m.OrderBook.ImbalancePct <= -10 {
		score += 4
	} else if m.OrderBook.ImbalancePct <= -5 {
		score += 2
	} else if m.OrderBook.ImbalancePct >= 30 {
		score -= 6
	} else if m.OrderBook.ImbalancePct >= 20 {
		score -= 4
	}

	// ------------------------------------------------------------
	// Spread
	// ------------------------------------------------------------

	if m.Ticker.LastPrice > 0 &&
		m.Ticker.Bid1Price > 0 &&
		m.Ticker.Ask1Price > 0 {

		spreadPct :=
			(m.Ticker.Ask1Price - m.Ticker.Bid1Price) /
				m.Ticker.LastPrice * 100

		if spreadPct <= 0.05 {
			score += 5
		} else if spreadPct <= 0.15 {
			score += 3
		} else if spreadPct > 0.5 {
			score -= 8
		}
	}

	// ------------------------------------------------------------
	// Дополнительный штраф
	// ------------------------------------------------------------

	if st1h.HighState == "LH" &&
		st1h.LowState == "HL" &&
		ok15m &&
		st15m.HighState == "HH" &&
		st15m.LowState == "HL" {

		score -= 10
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "confirmed impulse exhaustion near resistance + range + bearish structure",
	}
}

// change3d рассчитывает изменение цены примерно за последние 3 суток.
//
// Используем часовые свечи, поэтому 72 часа = 72 свечи.
//
// Формула:
//
//	(current close - close 72h ago) / close 72h ago * 100
//
// Если данных недостаточно, возвращаем 0.
//
// Важно: функция не предполагает, что candles отсортированы в каком-либо
// конкретном направлении. Для надёжности ищем самую старую и самую новую
// свечу и используем время свечей.
func change3d(candles []models.Candle) float64 {
	if len(candles) < 2 {
		return 0
	}

	latest := candles[0]
	earliest := candles[0]

	for _, candle := range candles[1:] {
		if candle.Time.Before(earliest.Time) {
			earliest = candle
		}

		if candle.Time.After(latest.Time) {
			latest = candle
		}
	}

	// Нам нужны данные минимум примерно за 3 суток.
	duration := latest.Time.Sub(earliest.Time)

	if duration < 72*60*60 {
		return 0
	}

	// Ищем свечу, которая максимально близка к моменту
	// "72 часа назад" от последней свечи.
	targetTime := latest.Time.Add(-72 * time.Hour)

	var reference models.Candle
	found := false

	for _, candle := range candles {
		if candle.Time.After(targetTime) {
			continue
		}

		if !found || candle.Time.After(reference.Time) {
			reference = candle
			found = true
		}
	}

	if !found || reference.Close <= 0 {
		return 0
	}

	return (latest.Close - reference.Close) / reference.Close * 100
}

// rejectShortGrid полностью исключает монету из кандидатов.
//
// Лучше получить 0 кандидатов, чем протащить сомнительную монету
// высоким score.
func rejectShortGrid(reason string) models.StrategyResult {
	return models.StrategyResult{
		Score:  0,
		Status: "reject",
		Reason: reason,
	}
}
