package strategies

import "universal-bybit-screener/models"

// NeutralGrid ищет рынок, в котором цена способна колебаться
// внутри относительно устойчивого диапазона без выраженного
// направленного движения.
//
// Важный принцип стратегии:
//
// 1. Сначала отсеиваем очевидно неподходящие рынки.
// 2. Только после этого рассчитываем score.
// 3. Score показывает качество среди уже подходящих кандидатов.
//
// Это важно для Neutral Grid:
// сильный тренд нельзя компенсировать хорошим стаканом,
// funding или ликвидностью.
type NeutralGrid struct{}

func (NeutralGrid) Name() string {
	return "neutral-grid"
}

func (NeutralGrid) Evaluate(
	m models.MarketData,
	i models.Indicators,
	s map[string]models.Structure,
	l models.Levels,
) models.StrategyResult {
	st15 := s["15m"]
	st1 := s["1h"]
	st4 := s["4h"]

	// -------------------------------------------------------------------------
	// 0. HARD REJECT
	//
	// Здесь не штрафуем score.
	// Если рынок принципиально не подходит для Neutral Grid,
	// сразу возвращаем reject.
	// -------------------------------------------------------------------------

	// Для Neutral Grid нужны обе границы диапазона.
	//
	// Если есть только support или только resistance,
	// у нас нет полноценного коридора для работы сетки.
	if l.NearestSupport <= 0 || l.NearestResistance <= 0 {
		return rejectNeutralGrid(
			"no complete support/resistance range",
		)
	}

	// Диапазон должен быть достаточно широким относительно ATR.
	//
	// Если диапазон меньше примерно 2.5 ATR,
	// обычное движение рынка способно быстро пройти
	// большую часть сетки.
	//
	// Это один из главных фильтров против "фальшивых диапазонов".
	if l.RangeToATR1h > 0 && l.RangeToATR1h < 2.5 {
		return rejectNeutralGrid(
			"range is too narrow relative to ATR",
		)
	}

	// Очень узкий абсолютный диапазон также не подходит.
	//
	// Например, диапазон 1.5-2% при ATR около 1.3%
	// практически не оставляет места для нормальной работы сетки.
	if l.RangeWidthPct > 0 && l.RangeWidthPct < 3 {
		return rejectNeutralGrid(
			"range is too narrow for neutral grid",
		)
	}

	// -------------------------------------------------------------------------
	// 0.1. Очевидный направленный тренд
	// -------------------------------------------------------------------------

	// Если одновременно 1h и 4h показывают один и тот же тренд,
	// Neutral Grid становится опасной:
	//
	// HH + HL = устойчивое движение вверх.
	// LH + LL = устойчивое движение вниз.
	//
	// Такой рынок не должен попадать в Neutral Grid только потому,
	// что цена временно находится между двумя уровнями.
	if isBullishStructure(st1) && isBullishStructure(st4) {
		return rejectNeutralGrid(
			"strong bullish trend on 1h and 4h",
		)
	}

	if isBearishStructure(st1) && isBearishStructure(st4) {
		return rejectNeutralGrid(
			"strong bearish trend on 1h and 4h",
		)
	}

	// Дополнительный фильтр:
	// если 4h показывает направленный тренд, а 3d и 7d
	// подтверждают движение в ту же сторону, это уже не просто
	// локальный шум — рынок действительно движется.
	change3d, ok3d := priceChangeFromCandles(m.Candles1h, 72)
	change7d, ok7d := priceChangeFromCandles(m.Candles1h, 168)

	if ok3d && ok7d {
		if isBullishStructure(st4) &&
			change3d > 10 &&
			change7d > 15 {
			return rejectNeutralGrid(
				"bullish momentum confirmed by 3d and 7d price movement",
			)
		}

		if isBearishStructure(st4) &&
			change3d < -10 &&
			change7d < -15 {
			return rejectNeutralGrid(
				"bearish momentum confirmed by 3d and 7d price movement",
			)
		}
	}

	// -------------------------------------------------------------------------
	// 0.2. Сильный краткосрочный импульс
	// -------------------------------------------------------------------------

	// Сильное движение за 24h — плохой фон для запуска нейтральной сетки.
	//
	// Особенно опасны значения > 10%, потому что рынок может продолжить
	// движение вместо возврата к среднему.
	change24h := m.Ticker.Price24hPcnt

	if abs(change24h) > 12 {
		return rejectNeutralGrid(
			"24h price movement is too strong",
		)
	}

	// Если одновременно 24h и 3d движение сильные и направлены одинаково,
	// это ещё один признак momentum market.
	if ok3d {
		if change24h > 8 && change3d > 10 {
			return rejectNeutralGrid(
				"short-term bullish momentum is too strong",
			)
		}

		if change24h < -8 && change3d < -10 {
			return rejectNeutralGrid(
				"short-term bearish momentum is too strong",
			)
		}
	}

	// -------------------------------------------------------------------------
	// 0.3. Экстремальный RSI
	// -------------------------------------------------------------------------

	// Neutral Grid предпочтительнее запускать в состоянии относительного
	// равновесия. Экстремальный RSI означает, что рынок уже сильно
	// смещён в одну сторону.
	//
	// Используем оба старших таймфрейма, чтобы не реагировать
	// на случайный всплеск только на одном TF.
	if i.RSI1h > 70 && i.RSI4h > 65 {
		return rejectNeutralGrid(
			"overbought momentum on 1h and 4h",
		)
	}

	if i.RSI1h < 30 && i.RSI4h < 35 {
		return rejectNeutralGrid(
			"oversold momentum on 1h and 4h",
		)
	}

	// -------------------------------------------------------------------------
	// 0.4. Экстремальная волатильность
	// -------------------------------------------------------------------------

	// ATR > 8% на 1h означает, что обычное часовое движение
	// уже слишком велико для спокойной нейтральной сетки.
	//
	// Мы не запрещаем умеренную волатильность:
	// именно она и нужна Grid.
	if i.ATR1hPct > 8 {
		return rejectNeutralGrid(
			"1h ATR is too high for neutral grid",
		)
	}

	// -------------------------------------------------------------------------
	// 0.5. Взрыв объёма
	// -------------------------------------------------------------------------

	// VolumeTrend показывает ускорение объёма.
	//
	// Значение > 3 означает очень сильное изменение активности.
	// Для Neutral Grid это подозрительно, потому что часто соответствует
	// импульсу, пробою или началу направленного движения.
	if i.VolumeTrend1h > 3 {
		return rejectNeutralGrid(
			"1h volume activity is too strong",
		)
	}

	// -------------------------------------------------------------------------
	// 0.6. Сильное изменение Open Interest
	// -------------------------------------------------------------------------

	oiChange, okOI := openInterestChange(m.OpenInterest)

	if okOI {
		// Резкое увеличение OI означает активное открытие новых позиций.
		//
		// В сочетании с движением цены это может быть началом нового
		// направленного движения.
		if oiChange > 20 {
			return rejectNeutralGrid(
				"open interest increased too strongly",
			)
		}

		// Сильное падение OI само по себе не обязательно плохо,
		// поэтому не делаем здесь симметричный hard reject.
	}

	// -------------------------------------------------------------------------
	// 0.7. Слишком широкий диапазон
	// -------------------------------------------------------------------------

	// Очень широкий диапазон часто означает не спокойный боковик,
	// а высоковолатильный рынок.
	//
	// До ~30% ещё может быть рабочий широкий Grid,
	// выше этого уже существенно возрастает риск выхода из диапазона.
	if l.RangeWidthPct > 35 {
		return rejectNeutralGrid(
			"range is too wide and volatile",
		)
	}

	// -------------------------------------------------------------------------
	// Если дошли сюда — рынок хотя бы прошёл базовые quality gates.
	// Теперь имеет смысл считать score.
	// -------------------------------------------------------------------------

	score := 50.0

	// -------------------------------------------------------------------------
	// 1. Полноценный диапазон
	// -------------------------------------------------------------------------

	score += 12

	// -------------------------------------------------------------------------
	// 2. Ширина диапазона
	// -------------------------------------------------------------------------

	switch {
	case l.RangeWidthPct >= 8 && l.RangeWidthPct <= 20:
		// Оптимальная зона:
		// диапазон достаточно большой для работы сетки,
		// но ещё не выглядит чрезмерно волатильным.
		score += 10

	case l.RangeWidthPct >= 5 && l.RangeWidthPct < 8:
		score += 6

	case l.RangeWidthPct >= 3 && l.RangeWidthPct < 5:
		score += 2

	case l.RangeWidthPct > 20 && l.RangeWidthPct <= 35:
		// Широкий диапазон допустим,
		// но риск выхода из него выше.
		score += 1
	}

	// -------------------------------------------------------------------------
	// 3. Диапазон относительно ATR
	// -------------------------------------------------------------------------

	switch {
	case l.RangeToATR1h >= 4 && l.RangeToATR1h <= 8:
		// Хорошее соотношение:
		// диапазон значительно шире обычного движения,
		// но не настолько огромен, чтобы выглядеть подозрительно.
		score += 12

	case l.RangeToATR1h >= 3 && l.RangeToATR1h < 4:
		score += 7

	case l.RangeToATR1h >= 2.5 && l.RangeToATR1h < 3:
		score += 3

	case l.RangeToATR1h > 8 && l.RangeToATR1h <= 12:
		// Очень широкий относительно ATR диапазон.
		// Это не плохо, но может означать, что уровни слишком далеко.
		score += 4

	case l.RangeToATR1h > 12:
		score -= 3
	}

	// -------------------------------------------------------------------------
	// 4. Положение цены внутри диапазона
	// -------------------------------------------------------------------------

	position := l.RangePositionPct

	switch {
	case position >= 40 && position <= 60:
		// Идеальная середина диапазона.
		score += 10

	case position >= 30 && position < 40:
		score += 7

	case position > 60 && position <= 70:
		score += 7

	case position >= 20 && position < 30:
		score += 3

	case position > 70 && position <= 80:
		score += 3

	default:
		// Цена слишком близко к границе.
		// Это увеличивает вероятность выхода из диапазона.
		score -= 5
	}

	// -------------------------------------------------------------------------
	// 5. Структура 1h
	// -------------------------------------------------------------------------

	switch {
	case st1.HighState == "EQ" || st1.LowState == "EQ":
		// Равенство максимумов/минимумов хорошо соответствует боковику.
		score += 8

	case isBullishStructure(st1) || isBearishStructure(st1):
		// Один TF показывает направленность,
		// поэтому качество Neutral Grid ниже.
		score -= 8

	default:
		// Смешанная структура может быть переходным состоянием
		// между трендом и диапазоном.
		score += 3
	}

	// -------------------------------------------------------------------------
	// 6. Структура 4h
	// -------------------------------------------------------------------------

	switch {
	case st4.HighState == "EQ" || st4.LowState == "EQ":
		score += 10

	case isBullishStructure(st4) || isBearishStructure(st4):
		// Мы не делаем hard reject здесь автоматически:
		// один направленный TF ещё не означает, что рынок полностью
		// непригоден. Но score существенно снижается.
		score -= 10

	default:
		score += 5
	}

	// -------------------------------------------------------------------------
	// 7. Структура 15m
	// -------------------------------------------------------------------------

	switch {
	case st15.HighState == "EQ" || st15.LowState == "EQ":
		score += 4

	case isBullishStructure(st15) || isBearishStructure(st15):
		// На младшем TF направленное движение допустимо,
		// если старшие TF остаются нейтральными.
		score -= 3

	default:
		score += 2
	}

	// -------------------------------------------------------------------------
	// 8. Изменение цены за 24h
	// -------------------------------------------------------------------------

	switch {
	case abs(change24h) <= 3:
		score += 5

	case abs(change24h) <= 5:
		score += 3

	case abs(change24h) <= 8:
		score += 1

	case abs(change24h) <= 12:
		score -= 5
	}

	// -------------------------------------------------------------------------
	// 9. Изменение цены за 3d и 7d
	// -------------------------------------------------------------------------

	if ok3d {
		switch {
		case abs(change3d) <= 8:
			score += 4

		case abs(change3d) <= 15:
			score += 1

		case abs(change3d) <= 25:
			score -= 5

		default:
			score -= 10
		}
	}

	if ok7d {
		switch {
		case abs(change7d) <= 12:
			score += 4

		case abs(change7d) <= 20:
			score += 1

		case abs(change7d) <= 35:
			score -= 5

		default:
			score -= 10
		}
	}

	// -------------------------------------------------------------------------
	// 10. RSI 1h
	// -------------------------------------------------------------------------

	switch {
	case i.RSI1h >= 45 && i.RSI1h <= 55:
		score += 6

	case i.RSI1h >= 40 && i.RSI1h < 45:
		score += 3

	case i.RSI1h > 55 && i.RSI1h <= 60:
		score += 3

	case i.RSI1h >= 35 && i.RSI1h < 40:
		score += 1

	case i.RSI1h > 60 && i.RSI1h <= 70:
		score -= 3

	case i.RSI1h >= 30 && i.RSI1h < 35:
		score -= 3

	default:
		score -= 5
	}

	// -------------------------------------------------------------------------
	// 11. RSI 4h
	// -------------------------------------------------------------------------

	switch {
	case i.RSI4h >= 45 && i.RSI4h <= 55:
		score += 6

	case i.RSI4h >= 40 && i.RSI4h < 45:
		score += 3

	case i.RSI4h > 55 && i.RSI4h <= 60:
		score += 3

	case i.RSI4h >= 35 && i.RSI4h < 40:
		score += 1

	case i.RSI4h > 60 && i.RSI4h <= 65:
		score -= 3

	case i.RSI4h >= 30 && i.RSI4h < 35:
		score -= 3

	default:
		score -= 5
	}

	// -------------------------------------------------------------------------
	// 12. ATR / волатильность
	// -------------------------------------------------------------------------

	switch {
	case i.ATR1hPct >= 1.5 && i.ATR1hPct <= 5:
		// Хорошая рабочая волатильность.
		score += 7

	case i.ATR1hPct >= 1 && i.ATR1hPct < 1.5:
		score += 4

	case i.ATR1hPct > 5 && i.ATR1hPct <= 8:
		// Работать можно, но риск выше.
		score += 2

	default:
		score -= 3
	}

	// -------------------------------------------------------------------------
	// 13. Объём
	// -------------------------------------------------------------------------

	if i.VolumeRatio1h >= 0.7 && i.VolumeRatio1h <= 1.5 {
		score += 4

	} else if i.VolumeRatio1h >= 0.5 && i.VolumeRatio1h < 0.7 {
		score += 1

	} else if i.VolumeRatio1h > 2 {
		score -= 4

	} else if i.VolumeRatio1h > 0 && i.VolumeRatio1h < 0.5 {
		score -= 2
	}

	if i.VolumeTrend1h >= 0.8 && i.VolumeTrend1h <= 1.5 {
		score += 4

	} else if i.VolumeTrend1h >= 0.5 && i.VolumeTrend1h < 0.8 {
		score += 1

	} else if i.VolumeTrend1h > 1.5 && i.VolumeTrend1h <= 3 {
		score -= 3

	} else if i.VolumeTrend1h > 0 && i.VolumeTrend1h < 0.5 {
		score -= 2
	}

	// -------------------------------------------------------------------------
	// 14. Funding
	// -------------------------------------------------------------------------

	funding := averageFunding24h(m.Funding)

	if len(m.Funding) > 0 {
		absFunding := abs(funding)

		switch {
		case absFunding <= 0.0001:
			score += 4

		case absFunding <= 0.0003:
			score += 1

		case absFunding <= 0.0007:
			score -= 3

		default:
			score -= 6
		}
	}

	// -------------------------------------------------------------------------
	// 15. Open Interest
	// -------------------------------------------------------------------------

	if okOI {
		absOI := abs(oiChange)

		switch {
		case absOI <= 5:
			score += 3

		case absOI <= 10:
			score += 1

		case absOI <= 20:
			score -= 3

		default:
			score -= 5
		}

		// Сильное движение цены + рост OI особенно опасны:
		// это часто означает формирование нового направленного движения.
		if ok3d && abs(change3d) > 15 && oiChange > 10 {
			score -= 6
		}
	}

	// -------------------------------------------------------------------------
	// 16. Order Book
	// -------------------------------------------------------------------------

	if m.OrderBook.BidNotional > 0 ||
		m.OrderBook.AskNotional > 0 {

		ratio := m.OrderBook.BidAskRatio
		imbalance := m.OrderBook.ImbalancePct

		if ratio >= 0.85 && ratio <= 1.18 {
			score += 4

		} else if ratio >= 0.75 && ratio <= 1.35 {
			score += 1

		} else if ratio > 1.5 ||
			(ratio > 0 && ratio < 0.67) {
			score -= 4
		}

		if imbalance >= -15 && imbalance <= 15 {
			score += 3

		} else if imbalance >= -25 && imbalance <= 25 {
			score += 1

		} else if imbalance > 35 || imbalance < -35 {
			score -= 4
		}
	}

	// -------------------------------------------------------------------------
	// 17. Spread
	// -------------------------------------------------------------------------

	if m.Ticker.Bid1Price > 0 &&
		m.Ticker.Ask1Price > 0 &&
		m.Ticker.LastPrice > 0 {

		spread := (m.Ticker.Ask1Price - m.Ticker.Bid1Price) /
			m.Ticker.LastPrice * 100

		switch {
		case spread <= 0.05:
			score += 4

		case spread <= 0.15:
			score += 2

		case spread <= 0.30:
			score -= 2

		default:
			score -= 7
		}
	}

	// -------------------------------------------------------------------------
	// 18. Финальная защита от directional market
	// -------------------------------------------------------------------------

	// Если после всех проверок всё равно одновременно наблюдается
	// направленная структура + сильное движение за 3d/7d,
	// дополнительно снижаем score.
	//
	// Это не hard reject, потому что структура может быть неоднозначной,
	// но кандидат явно хуже настоящего боковика.
	if isBullishStructure(st4) &&
		ok3d &&
		ok7d &&
		change3d > 10 &&
		change7d > 15 {

		score -= 8
	}

	if isBearishStructure(st4) &&
		ok3d &&
		ok7d &&
		change3d < -10 &&
		change7d < -15 {

		score -= 8
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "range quality + multi-timeframe neutrality + controlled volatility + liquidity",
	}
}

// rejectNeutralGrid возвращает единый результат для hard reject.
//
// Score = 0 принципиально отличается от низкого score:
// рынок не просто "плохой среди кандидатов", он не соответствует
// самой идее Neutral Grid.
func rejectNeutralGrid(reason string) models.StrategyResult {
	return models.StrategyResult{
		Score:  0,
		Status: "reject",
		Reason: reason,
	}
}

// priceChangeFromCandles рассчитывает изменение цены за указанное
// количество часовых свечей.
//
// Например:
// 72 свечи ≈ 3 дня.
// 168 свечей ≈ 7 дней.
//
// Используется закрытая история.
func priceChangeFromCandles(
	candles []models.Candle,
	period int,
) (float64, bool) {
	if len(candles) <= period {
		return 0, false
	}

	last := candles[len(candles)-1].Close
	first := candles[len(candles)-1-period].Close

	if first <= 0 || last <= 0 {
		return 0, false
	}

	return (last - first) / first * 100, true
}

// averageFunding24h рассчитывает среднее funding
// по переданной истории.
func averageFunding24h(
	funding []models.FundingPoint,
) float64 {
	if len(funding) == 0 {
		return 0
	}

	var sum float64

	for _, point := range funding {
		sum += point.Rate
	}

	return sum / float64(len(funding))
}

// openInterestChange рассчитывает изменение OI
// между первой и последней доступной точкой.
func openInterestChange(
	points []models.OpenInterestPoint,
) (float64, bool) {
	if len(points) < 2 {
		return 0, false
	}

	first := points[0].OpenInterest
	last := points[len(points)-1].OpenInterest

	if first <= 0 {
		return 0, false
	}

	return (last - first) / first * 100, true
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}

	return v
}

func isBullishStructure(st models.Structure) bool {
	return st.HighState == "HH" && st.LowState == "HL"
}

func isBearishStructure(st models.Structure) bool {
	return st.HighState == "LH" && st.LowState == "LL"
}
