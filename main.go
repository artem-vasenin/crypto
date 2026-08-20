package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"cs/analysis"
	"cs/bybit"
	"cs/indicators"
	"cs/model"
)

const (
	// Главный фильтр нашего проекта.
	maxPrice = 2.0

	// Минимальный 24h оборот.
	minTurnover24h = 5_000_000

	// Сколько кандидатов показываем в консоли.
	topCandidates = 20

	// Количество одновременно работающих workers.
	//
	// Не стоит делать его огромным:
	// мы не хотим упираться в rate limits Bybit.
	workerCount = 8

	// Сколько свечей запрашиваем.
	//
	// 200 свечей 15m = примерно 50 часов.
	// Этого достаточно для RSI/ATR/структуры.
	candles15m = 200
	candles1h  = 200
	candles4h  = 200
)

type marketData struct {
	Candles15m []model.Candle
	Candles1h  []model.Candle
	Candles4h  []model.Candle

	Funding []float64
	OI      []float64
}

// result используется worker pool'ом.
type result struct {
	Candidate model.Candidate
	Err       error
	Symbol    string
}

func main() {

	// Контекст с timeout на весь запуск скринера.
	//
	// Если Bybit внезапно зависнет,
	// программа не должна работать бесконечно.
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	client := bybit.NewClient()

	log.Println("Получаем список инструментов Bybit...")

	instruments, err :=
		client.GetInstruments(ctx)

	if err != nil {
		log.Fatalf(
			"ошибка получения инструментов: %v",
			err,
		)
	}

	log.Printf(
		"Получено инструментов: %d\n",
		len(instruments),
	)

	log.Println("Получаем тикеры...")

	tickers, err :=
		client.GetTickers(ctx)

	if err != nil {
		log.Fatalf(
			"ошибка получения тикеров: %v",
			err,
		)
	}

	// -------------------------------------------------------------------------
	// Первичная фильтрация
	// -------------------------------------------------------------------------

	var candidates []model.Candidate

	for _, instrument := range instruments {

		ticker, ok :=
			tickers[instrument.Symbol]

		if !ok {
			continue
		}

		// Цена должна быть <= $2.
		if ticker.LastPrice > maxPrice {
			continue
		}

		// Оборот должен быть >= $5M.
		if ticker.Turnover24h < minTurnover24h {
			continue
		}

		// Нулевая цена — явно плохие данные.
		if ticker.LastPrice <= 0 {
			continue
		}

		// Для нормального рынка bid/ask должны существовать.
		if ticker.BidPrice <= 0 ||
			ticker.AskPrice <= 0 {
			continue
		}

		spread :=
			(ticker.AskPrice/ticker.BidPrice - 1) * 100

		candidates = append(
			candidates,
			model.Candidate{
				Symbol: ticker.Symbol,

				Price:       ticker.LastPrice,
				Change24h:   ticker.Change24h,
				Turnover24h: ticker.Turnover24h,

				FundingRate: ticker.FundingRate,

				OpenInterest:      ticker.OpenInterest,
				OpenInterestValue: ticker.OpenInterestValue,

				SpreadPercent: spread,
			},
		)
	}

	// Сначала самые сильно выросшие монеты.
	sort.Slice(
		candidates,
		func(i, j int) bool {
			return candidates[i].Change24h >
				candidates[j].Change24h
		},
	)

	fmt.Printf(
		"\nКандидаты: цена <= $%.2f, оборот >= $%.2fM\n\n",
		maxPrice,
		minTurnover24h/1_000_000,
	)

	fmt.Println(
		"ТОП-20 по росту за 24 часа:",
	)
	fmt.Println(
		"--------------------------------------------------------------------------------",
	)

	displayCount := min(
		topCandidates,
		len(candidates),
	)

	for i := 0; i < displayCount; i++ {

		candidate := candidates[i]

		fmt.Printf(
			"%2d. %-18s price: %-12.8f 24h: %+7.2f%% turnover: %.0f USDT\n",
			i+1,
			candidate.Symbol,
			candidate.Price,
			candidate.Change24h,
			candidate.Turnover24h,
		)
	}

	fmt.Printf(
		"\nВсего кандидатов после фильтрации: %d\n\n",
		len(candidates),
	)

	// -------------------------------------------------------------------------
	// Подготавливаем worker pool
	// -------------------------------------------------------------------------

	jobs := make(chan model.Candidate)

	results := make(chan result)

	var wg sync.WaitGroup

	// Запускаем ограниченное количество workers.
	for i := 0; i < workerCount; i++ {

		wg.Add(1)

		go func() {
			defer wg.Done()

			for candidate := range jobs {

				log.Printf(
					"[%s] анализ...",
					candidate.Symbol,
				)

				analyzed, err :=
					analyzeCandidate(
						ctx,
						client,
						candidate,
					)

				results <- result{
					Candidate: analyzed,
					Err:       err,
					Symbol:    candidate.Symbol,
				}
			}
		}()
	}

	// Отдельная goroutine закрывает results,
	// когда workers закончили работу.
	go func() {

		wg.Wait()

		close(results)

	}()

	// Отправляем кандидатов workers.
	go func() {

		defer close(jobs)

		for _, candidate := range candidates {

			select {

			case jobs <- candidate:

			case <-ctx.Done():
				return
			}
		}
	}()

	// -------------------------------------------------------------------------
	// Собираем результаты
	// -------------------------------------------------------------------------

	finalCandidates := make(
		[]model.Candidate,
		0,
		len(candidates),
	)

	for result := range results {

		if result.Err != nil {

			log.Printf(
				"[%s] ошибка: %v",
				result.Symbol,
				result.Err,
			)

			continue
		}

		finalCandidates =
			append(
				finalCandidates,
				result.Candidate,
			)
	}

	// Сортируем пока просто по 24h росту.
	//
	// Финальный score подключим после того,
	// как убедимся, что все индикаторы корректно считаются.
	sort.Slice(
		finalCandidates,
		func(i, j int) bool {
			return finalCandidates[i].Change24h >
				finalCandidates[j].Change24h
		},
	)

	// -------------------------------------------------------------------------
	// JSON
	// -------------------------------------------------------------------------

	report := model.Report{
		GeneratedAt: time.Now().UTC().Format(
			time.RFC3339,
		),

		TotalInstruments: len(instruments),

		TotalCandidates: len(finalCandidates),

		Candidates: finalCandidates,
	}

	report.Filters.MaxPrice =
		maxPrice

	report.Filters.MinTurnover24h =
		minTurnover24h

	if err := saveJSON(
		"report.json",
		report,
	); err != nil {

		log.Fatalf(
			"ошибка сохранения JSON: %v",
			err,
		)
	}

	fmt.Println()
	fmt.Println(
		"==============================================",
	)
	fmt.Println(
		"АНАЛИЗ ЗАВЕРШЁН",
	)
	fmt.Println(
		"==============================================",
	)

	fmt.Printf(
		"Обработано кандидатов: %d\n",
		len(finalCandidates),
	)

	fmt.Println(
		"Результат сохранён в report.json",
	)

	// Выводим короткий итог.
	printFinalResults(finalCandidates)
}

// analyzeCandidate загружает все необходимые данные
// и рассчитывает индикаторы.
//
// Эта функция вызывается workers.
//
// Именно поэтому мы можем анализировать несколько монет
// одновременно.
func analyzeCandidate(
	ctx context.Context,
	client *bybit.Client,
	candidate model.Candidate,
) (model.Candidate, error) {

	// -------------------------------------------------------------------------
	// Получаем свечи
	// -------------------------------------------------------------------------

	candles15m, err :=
		client.GetKlines(
			ctx,
			candidate.Symbol,
			"15",
			candles15m,
		)

	if err != nil {
		return candidate, err
	}

	candles1h, err :=
		client.GetKlines(
			ctx,
			candidate.Symbol,
			"60",
			candles1h,
		)

	if err != nil {
		return candidate, err
	}

	candles4h, err :=
		client.GetKlines(
			ctx,
			candidate.Symbol,
			"240",
			candles4h,
		)

	if err != nil {
		return candidate, err
	}

	// -------------------------------------------------------------------------
	// Funding
	// -------------------------------------------------------------------------

	funding, err :=
		client.GetFundingHistory(
			ctx,
			candidate.Symbol,
			10,
		)

	if err != nil {
		return candidate, err
	}

	// -------------------------------------------------------------------------
	// Open Interest
	// -------------------------------------------------------------------------

	oi, err :=
		client.GetOpenInterest(
			ctx,
			candidate.Symbol,
			25,
		)

	if err != nil {
		return candidate, err
	}

	// -------------------------------------------------------------------------
	// Индикаторы
	// -------------------------------------------------------------------------

	candidate.Indicators = model.IndicatorData{

		RSI15m: indicators.RSI(
			candles15m,
			14,
		),

		RSI1h: indicators.RSI(
			candles1h,
			14,
		),

		RSI4h: indicators.RSI(
			candles4h,
			14,
		),

		ATR15m: indicators.ATR(
			candles15m,
			14,
		),

		ATR1h: indicators.ATR(
			candles1h,
			14,
		),

		ATR4h: indicators.ATR(
			candles4h,
			14,
		),

		ATR15mPercent: indicators.ATRPercent(
			candles15m,
			14,
		),

		ATR1hPercent: indicators.ATRPercent(
			candles1h,
			14,
		),

		ATR4hPercent: indicators.ATRPercent(
			candles4h,
			14,
		),

		VolumeRatio15m: indicators.VolumeRatio(
			candles15m,
			20,
		),

		VolumeRatio1h: indicators.VolumeRatio(
			candles1h,
			20,
		),

		VolumeRatio4h: indicators.VolumeRatio(
			candles4h,
			20,
		),
	}

	// -------------------------------------------------------------------------
	// Структура
	// -------------------------------------------------------------------------

	candidate.Structure4h =
		analysis.BuildStructure(
			"4H",
			candles4h,
			candidate.Price,
		)

	candidate.Structure1h =
		analysis.BuildStructure(
			"1H",
			candles1h,
			candidate.Price,
		)

	candidate.Structure15m =
		analysis.BuildStructure(
			"15m",
			candles15m,
			candidate.Price,
		)

	// -------------------------------------------------------------------------
	// OI analysis
	// -------------------------------------------------------------------------

	if len(oi) >= 2 {

		current := oi[len(oi)-1]
		previous := oi[0]

		change := 0.0

		if previous != 0 {
			change =
				(current/previous - 1) * 100
		}

		candidate.OI = model.OIAnalysis{
			Current:       current,
			ChangePercent: change,
		}
	}

	// -------------------------------------------------------------------------
	// 7d change
	// -------------------------------------------------------------------------

	// Для 7d нам достаточно взять 1h свечи.
	//
	// 200 часов > 7 суток.
	if len(candles1h) >= 169 {

		oldPrice :=
			candles1h[len(candles1h)-169].Close

		if oldPrice > 0 {

			candidate.Change7d =
				(candidate.Price/oldPrice - 1) * 100
		}
	}

	// -------------------------------------------------------------------------
	// Resistance
	// -------------------------------------------------------------------------

	candidate.NearestResistance =
		findResistance(
			candidate.Price,
			candidate.Structure4h,
			candidate.Structure1h,
			candidate.Structure15m,
		)

	if candidate.NearestResistance > 0 {

		candidate.ResistanceDistancePercent =
			(candidate.NearestResistance/candidate.Price - 1) *
				100
	}

	// Funding нам пригодится в будущем
	// для отдельного scoring.
	//
	// Пока тикер уже содержит текущий funding,
	// поэтому историю просто загружаем,
	// чтобы JSON мог расшириться в дальнейшем.
	_ = funding

	return candidate, nil
}

// findResistance ищет ближайший pivot high,
// который находится выше текущей цены.
func findResistance(
	price float64,
	structures ...model.Structure,
) float64 {

	var nearest float64

	for _, structure := range structures {

		for _, pivot := range structure.PivotHighs {

			if pivot.Price <= price {
				continue
			}

			if nearest == 0 ||
				pivot.Price < nearest {

				nearest = pivot.Price
			}
		}
	}

	return nearest
}

// saveJSON сохраняет отчёт.
//
// json.MarshalIndent используется специально:
// файл должен быть удобным для чтения человеком
// и одновременно легко обрабатываться программой/AI.
func saveJSON(
	filename string,
	report model.Report,
) error {

	data, err :=
		json.MarshalIndent(
			report,
			"",
			"  ",
		)

	if err != nil {
		return err
	}

	// Добавляем перенос строки в конец файла.
	data = append(data, '\n')

	return os.WriteFile(
		filename,
		data,
		0644,
	)
}

// printFinalResults выводит короткий результат.
func printFinalResults(
	candidates []model.Candidate,
) {

	fmt.Println()
	fmt.Println(
		"ТОП КАНДИДАТОВ ПО 24H:",
	)
	fmt.Println(
		"--------------------------------------------------------------------------------",
	)

	count := min(
		20,
		len(candidates),
	)

	for i := 0; i < count; i++ {

		candidate := candidates[i]

		fmt.Printf(
			"%2d. %-18s price: %.8f 24h: %+7.2f%% 7d: %+7.2f%% RSI1h: %5.1f RSI4h: %5.1f ATR1h: %5.2f%% resistance: %.8f\n",
			i+1,
			candidate.Symbol,
			candidate.Price,
			candidate.Change24h,
			candidate.Change7d,
			candidate.Indicators.RSI1h,
			candidate.Indicators.RSI4h,
			candidate.Indicators.ATR1hPercent,
			candidate.NearestResistance,
		)
	}
}

// min возвращает меньшее из двух чисел.
func min(a, b int) int {

	if a < b {
		return a
	}

	return b
}
