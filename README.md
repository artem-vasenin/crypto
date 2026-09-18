# Bybit Universal Deep Coin Analyzer v3.2

Универсальный evidence-first анализатор публичных данных Bybit V5 для второго этапа исследования кандидатов. Программа собирает глубокую историю, рассчитывает нейтральные признаки для GRID и обычных directional LONG/SHORT позиций и сохраняет **один самодостаточный JSON на символ**. Финальное решение OPEN/WAIT/NO-GRID/REJECT программа намеренно не принимает: его должен делать внешний ИИ или человек после проверки raw evidence.

## Главное в v3.2

- Интерактивный выбор: `GRID`, `Directional` или `Полный анализ`; **Enter = Полный анализ**.
- Направление: `LONG`, `SHORT`, `LONG + SHORT`; **Enter = LONG + SHORT**.
- 1–10 символов через запятую без пробелов: `BTC,ETH,SOL`; сокращения автоматически становятся `BTCUSDT`.
- Предварительная проверка символов до тяжёлой загрузки.
- Raw evidence сохраняется **всегда**, отдельного переключателя нет.
- Независимые `market_regime`, `grid_analysis` и `directional_analysis`.
- Противоположные bullish/bearish/conflicting evidence не скрываются даже при выборе одного направления.
- После анализа Enter запускает новый сеанс.

## Требования и запуск

Требуется Go 1.23+ и доступ к публичному Bybit V5 API. API-ключ не нужен.

```bash
go test ./...
go build -o coin-analyzer ./cmd/analyzer
./coin-analyzer
```

Windows PowerShell:

```powershell
go test ./...
go build -o coin-analyzer.exe ./cmd/analyzer
.\coin-analyzer.exe
```

Отчёты создаются в `reports/` рядом с бинарником.

## Диалог

```text
Что анализируем?
1 — GRID BOT
2 — Обычная позиция (Directional)
3 — Полный анализ (GRID + Directional) [по умолчанию]
Ваш выбор [3]:

Какое направление анализируем?
1 — LONG
2 — SHORT
3 — LONG + SHORT [по умолчанию]
Ваш выбор [3]:

Введите 1-10 символов через запятую БЕЗ пробелов:
> BTC,ETH,SOL
```

Пустой Enter на первых двух вопросах означает `3`. Неверный символ не останавливает обработку остальных. После сеанса `Enter`/`1` — новый анализ, `0` — выход.

## Глубина данных

Целевые объёмы OHLCV: 5m ≈ 72 часа (864 свечи), 15m ≈ 14 дней (1344), 1h ≈ 60 дней (1440), 4h ≈ 180 дней (1080), 1D ≈ 365 дней (365). Клиент использует пагинацию там, где одного ответа API недостаточно. Дополнительно собираются funding history, OI 5m ≈ 24h, long/short ratio 5m ≈ 24h, стакан до 500 уровней, до 1000 последних public trades, mark/index history и BTC-контекст.

Фактическое число записей и предупреждения записываются в `data_quality`. Неполный secondary endpoint не уничтожает весь отчёт; критически недостаточная основная OHLCV-история останавливает отчёт по конкретному символу.

## Структура JSON

- `request` — что выбрал пользователь; это контекст, а не приказ алгоритму искать подтверждение.
- `data_quality` — полнота, warnings, фактические counts и целевая глубина.
- `market` — текущий snapshot.
- `timeframes` — RSI/ATR/ADX/EMA/Efficiency Ratio/realized vol/Bollinger width/volume ratio и геометрия окна.
- `market_regime` — нейтральное описание ranging/trending/compression/expansion/breakout risk.
- `range_analysis` — границы, midpoint, touches, midpoint crosses, false breaks, slope.
- `grid_analysis` — пригодность режима для grid, range/mean-reversion quality, breakout risk и независимые LONG/SHORT evidence.
- `directional_analysis` — независимые LONG/SHORT evidence для обычной позиции, конфликты, контекст invalidation/targets.
- `derivatives` — funding/OI/long-short history и агрегаты.
- `microstructure` — order book, taker delta, mark/index отклонения.
- `btc_context` — корреляция и относительная сила к BTC.
- `raw_evidence` — полные OHLCV, recent trades, mark/index history. Сохраняется всегда.
- `ai_instructions` — подсказка внешнему ИИ: перепроверять derived metrics по raw evidence и соблюдать иерархию evidence.

## Проверка проекта

Перед использованием выполнить:

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build ./cmd/analyzer
```

Для live smoke-test запустить один ликвидный символ (`BTC`), дождаться JSON и проверить: `generated_at` свежий; `data_quality.counts` разумны; OHLCV отсортирован по времени; `raw_evidence` не пуст; `request` соответствует меню; JSON открывается стандартным parser. Ошибки сети/лимитов Bybit должны появляться в консоли и/или `data_quality.warnings`, а не маскироваться.

## Все файлы и функции

### `cmd/analyzer/main.go`
Интерактивный CLI.
- `getExeDir` — находит директорию бинарника.
- `normalizeSymbol` — нормализует `BTC` → `BTCUSDT`.
- `parseSymbols` — валидирует список, удаляет дубликаты, максимум 10.
- `askChoice` — универсальное меню с выбором по Enter.
- `chooseRequest` — собирает тип анализа и направление.
- `saveReport` — сохраняет timestamped JSON.
- `analyzeSession` — проверяет символы и запускает глубокий анализ.
- `askContinue` — новый сеанс/выход.
- `main` — точка входа.

### `internal/analysis/types.go`
Содержит структуры JSON: `Request`, `Report`, `DataQuality`, `Market`, `Timeframe`, `MarketRegime`, `RangeAnalysis`, `SideAssessment`, `GridAnalysis`, `DirectionalAnalysis`, `Derivatives`, `Microstructure`, `BTCContext`, `RawEvidence`, `AIInstructions`. Функций нет.

### `internal/analysis/build.go`
Оркестрация и derived analysis.
- `closes`, `vols`, `ic` — преобразование свечей в массивы/тип indicators.
- `last`, `pct`, `min`, `mean`, `tail` — вспомогательная математика.
- `tf` — метрики одного timeframe.
- `analyzeRange` — характеристики диапазона.
- `classifyRegime` — market regime и GRID diagnostics без OPEN-сигнала.
- `analyzeDirectional` — независимая LONG/SHORT directional диагностика.
- Вспомогательные математические функции — безопасные расчёты процентов, окон и границ без интегральных score.
- `oiChange` — изменение OI за заданное число точек.
- `corrReturns` — корреляция доходностей.
- `Build` — полный pipeline одного символа.
- `timeNowUTC` — единый UTC timestamp.

### `internal/bybit/client.go`
HTTP-клиент публичного Bybit V5.
- `NewClient` — создаёт клиент.
- `(*Client).get` — общий GET, обработка HTTP/Bybit ошибок.
- `f64`, `i64` — преобразование строк API в числа.
- `(*Client).Ticker` — ticker snapshot/валидация символа.
- `(*Client).KlinesRange` — OHLCV с пагинацией.
- `(*Client).priceKlinesRange` — общий загрузчик mark/index klines.
- `(*Client).MarkKlinesRange` — mark-price history.
- `(*Client).IndexKlinesRange` — index-price history.
- `(*Client).Funding` — funding history.
- `(*Client).OpenInterest` — OI history.
- `(*Client).LongShort` — account long/short ratio history.
- `(*Client).OrderBook` — стакан.
- `(*Client).RecentTrades` — последние публичные сделки/taker side.

### `internal/indicators/indicators.go`
Чистая математика индикаторов.
- `Mean`, `Std` — среднее и стандартное отклонение.
- `EMA` — экспоненциальная средняя.
- `RSI` — RSI.
- `ATR` — Average True Range.
- `PercentChange` — процентное изменение.
- `Correlation` — корреляция Pearson.
- `EfficiencyRatio` — направленность движения относительно шума.
- `RealizedVolPct` — realized volatility.
- `BollingerWidthPct` — ширина Bollinger Bands.
- `ADX` — сила тренда.

### `internal/indicators/indicators_test.go`
Unit tests: `TestEMA`, `TestEfficiencyRatio`, `TestRSI`.

### `internal/output/json.go`
- `WriteJSON` — JSON encoder с optional pretty-print.

### `go.mod`
Модуль Go и версия языка; функций нет.

### `.gitignore`
Исключает бинарники, отчёты и служебные файлы.

### `AGENTS.md`
Полный handoff для ИИ/разработчика: цели, архитектура, инварианты, JSON, алгоритмы и правила расширения.

## Важные ограничения

Интегральные score намеренно отсутствуют: признаки сохраняются по иерархии значимости. Public trades — короткий snapshot, не полноценный исторический CVD. Стакан — моментальный снимок. Support/resistance/range — алгоритмические признаки, которые внешний ИИ должен перепроверять по raw OHLCV. При изменении Bybit API сначала обновляется клиент и тесты, затем аналитика.

---

## Изменения версии 3.1 — evidence hierarchy вместо score

В 3.1 полностью удалены интегральные `score`, `suitability_score`, `range_quality` и другие числовые рейтинги, которые складывали разнородные признаки. Причина: RSI, структура 4h, breakout, OI и положение в диапазоне имеют разную причинную значимость, а произвольная арифметическая сумма создаёт ложную точность.

Вместо этого аналитика строится иерархически:

1. **Hard blocks** — условия, которые способны запретить GRID независимо от вторичных плюсов: дрейф диапазона, высокий breakout risk, сильный эффективный тренд.
2. **Primary evidence** — ключевые признаки направления: multi-TF swing structure и другие наиболее содержательные факторы.
3. **Secondary evidence** — контекстные подтверждения, которые не должны перевешивать primary evidence.
4. **Risk factors / conflicts** — признаки, требующие осторожности или внешней проверки.
5. **Raw evidence** — всегда сохраняется и имеет приоритет перед ошибочным derived label.

Добавлены: подтверждённые swing points с timestamp для 5m/15m/1h/4h; rolling stationarity/drift range; история режима; OI 15m/1h/4h/24h; price+OI state; funding 24h/7d и percentile; long-ratio changes 1h/4h/24h; фактическое временное покрытие последних сделок и taker-flow окна 1m/5m/15m; mark/index history увеличена до 60 минут. Order book по-прежнему честно помечается как snapshot.

### Важное правило интерпретации

Поля `state`, `regime`, `primary_evidence`, `secondary_evidence`, `hard_blocks` и `risk_factors` — диагностические данные, а не команда на сделку. Финальные OPEN / WAIT / NO-GRID / REJECT, entry, grid range, SL и targets определяет внешний ИИ/человек после проверки raw evidence.

## Актуальный перечень ключевых функций v3.2

### `cmd/analyzer/main.go`
- `main` — основной интерактивный цикл программы.
- `askAnalysisType` — выбор GRID / Directional / полного анализа; Enter выбирает полный.
- `askDirection` — выбор LONG / SHORT / обоих; Enter выбирает оба.
- `askSymbols` — ввод, нормализация и ограничение списка символов десятью.
- `runSession` — проверка символов, последовательный deep analysis и сохранение JSON.
- `askContinue` — новый сеанс или завершение.

### `internal/analysis/build.go`
- `tf` — нейтральные метрики таймфрейма.
- `analyzeRange` — 48h range, rolling drift/width и stationarity.
- `swings` — подтверждённые pivot high/low с координатами.
- `structure` — HH/HL/LH/LL на основе реальных swings.
- `classifyRegime` — иерархическая regime/grid диагностика без score.
- `analyzeDirectional` — evidence-first LONG/SHORT диагностика без score.
- `oiChange`, `lsChange`, `priceOIState` — multi-window derivatives evidence.
- `avgFunding`, `percentile` — контекст funding.
- `flow` — taker-flow на доступных временных окнах.
- `regimeHistory` — rolling история изменения режима.
- `Build` — сбор всех данных и построение одного самодостаточного отчёта.

### `internal/bybit/client.go`
REST-клиент публичного Bybit V5: ticker, OHLCV с пагинацией, mark/index candles, funding, OI, long/short ratio, order book и public trades.

### `internal/indicators/indicators.go`
Чистые математические функции: Mean, Std, EMA, RSI, ATR, ADX, Efficiency Ratio, realized volatility, Bollinger width, percent change и correlation.

### `internal/output/json.go`
Сериализация отчёта в человекочитаемый JSON и безопасная запись файла.

## Изменения v3.2 после live-теста BTC/ETH

Версия 3.2 исправляет три проблемы, обнаруженные при сравнительном live-тесте v3.1 на BTCUSDT и ETHUSDT.

1. **Taker-flow больше не притворяется полным временным окном.** REST endpoint recent public trades для активных linear-инструментов может вернуть 1000 сделок всего за несколько десятков секунд. Для окон `1m`, `5m`, `15m` теперь сохраняются `available`, `status`, требуемое и фактическое покрытие. Delta рассчитывается и используется directional-анализом только если выборка реально покрывает всё окно. Короткий snapshot остаётся в raw evidence и общих snapshot-полях, но не считается 1m/5m/15m сигналом.
2. **Режим range получил семантику стационарности.** Вместо неоднозначного `RANGING` используются `RANGING_STATIONARY`, `RANGING_DRIFT_UP`, `RANGING_DRIFT_DOWN` (или `RANGING_DRIFT` при нулевом знаке), если более приоритетный trending/breakout/compression режим не сработал.
3. **Outside-bar swings явно маркируются.** Свеча, которая одновременно является pivot high и pivot low, сохраняется в `last_swings` с `ambiguous_outside_bar=true`, но исключается из вычисления HH/HL/LH/LL. Это не позволяет одной широкой свече искусственно сформировать структуру.

### Проверка исправлений

```bash
go test ./...
go vet ./...
go build ./cmd/analyzer
```

Для сравнительного live-теста рекомендуется снова запустить режим по умолчанию на `BTC,ETH`. На очень активном рынке ожидаемо, что `recent_trades` может покрывать меньше минуты; в этом случае `taker_flow_windows` честно покажет `insufficient_data`, а directional evidence не будет использовать краткий snapshot как минутный сигнал.
