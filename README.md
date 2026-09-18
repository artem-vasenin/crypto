# Candidate Screener

Независимый скриннер кандидатов рынка **Bybit USDT Perpetuals** на Go. Проект предназначен для первичного массового отбора монет перед анализом ИИ и отдельным Deep Coin Analyzer. Он **не торгует**, не создаёт ботов, не управляет позициями и не содержит execution layer.

## Основная идея

Один общий движок собирает публичные данные Bybit и формирует `MarketSnapshot`: направление, силу, динамику, MTF-согласованность и объективные features. Начиная с v1.1 Grid дополнительно получает rolling equilibrium features: миграцию midpoint, изменение ширины range, число пересечений midpoint и долю значимых возвратов к центру. После этого snapshot независимо интерпретируется профилем FUTURES или GRID. Единого score нет: допуск построен как иерархия hard gates, primary evidence, confirmations и risk flags.

## Требования

- Go 1.23+
- доступ к `https://api.bybit.com`
- API key не нужен: используются публичные V5 market endpoints.

## Быстрый запуск

```bash
go test ./...
go run ./cmd/screener
```

Без флагов запускается автоматический **FUTURES screening** и создаются:

- `output/screening-long.json`
- `output/screening-short.json`

GRID:

```bash
go run ./cmd/screener --mode grid
```

Создаются:

- `output/screening-grid-long.json`
- `output/screening-grid-short.json`

Ручное меню:

```bash
go run ./cmd/screener --interactive
```

Другой конфиг:

```bash
go run ./cmd/screener --config ./my-config.json --mode grid
```

## Семантика результата

`CANDIDATE` означает «монета заслуживает более глубокого анализа», а не «открыть позицию». Система не добирает кандидатов до квоты. Пустой `candidates: []` — корректный результат. Файл всё равно создаётся, поэтому отсутствие файла можно трактовать как ошибку запуска.

## Конфигурация

`configs.json` задаёт URL Bybit, output directory, минимальный 24h turnover, глубину истории, concurrency, reference capital для Grid и thresholds. Thresholds не являются весами score: это независимые границы hard/primary правил. Grid thresholds отдельно ограничивают rolling midpoint drift, range expansion, directional efficiency и требуют минимальное качество mean reversion.

`grid_max_price_usdt` — дополнительный практический ограничитель v1. Главная capital-проверка использует `minNotional/minOrderQty` и reference capital; в дальнейшем абсолютный price cap можно ослабить/удалить после валидации.

## Выходной JSON

Каждый файл содержит `schema_version`, `screener_version`, время генерации, режим, направление и массив объяснимых кандидатов. У кандидата сохраняются direction/strength/dynamics/MTF, primary evidence, confirmations, risk flags и несколько метрик для ИИ.

## Структура и API файлов

Ниже перечислены **все исходные файлы проекта** и функции/структуры, определённые в каждом из них.

### `cmd/screener/main.go`
Точка входа и orchestration; бизнес-математики здесь нет.
- `main()` — разбирает `--mode`, `--interactive`, `--config`, запускает приложение.
- `menu()` — ручное меню FUTURES/GRID.
- `run(mode, cfgPath)` — получает universe, параллельно строит snapshots, вызывает нужный analyzer и пишет два JSON.

### `internal/config/config.go`
Конфигурация и калибруемые thresholds.
- `Thresholds` — независимые границы drift/ADX/efficiency/expansion и MTF hard block.
- `Config` — эксплуатационные параметры Bybit, universe, concurrency, Grid capital.
- `Load(path)` — читает JSON, применяет безопасные defaults и валидирует критические поля.

### `internal/domain/types.go`
Общие типы данных без бизнес-логики.
- `Candle` — OHLCV.
- `Instrument` — symbol и торговые ограничения Bybit.
- `Ticker` — price/turnover/volume/funding/OI.
- `Direction` — UP/DOWN/SIDEWAYS/TRANSITION/UNKNOWN.
- `Strength` — WEAK/MODERATE/STRONG/UNKNOWN.
- `Dynamics` — ACCELERATING/STABLE/DECELERATING/UNKNOWN.
- `StructureState` — HH_HL/LH_LL/MIXED/UNKNOWN.
- `TimeframeFeatures` — признаки отдельного TF, включая rolling midpoint drift, изменение ширины range, midpoint crossings и mean-reversion ratio.
- `MarketSnapshot` — единое объективное описание монеты до strategy interpretation.
- `Candidate` — объяснимый допущенный кандидат.
- `ScreeningFile` — контракт выходного JSON.

### `internal/bybit/client.go`
Минимальный публичный Bybit V5 REST client.
- `Client` — base URL и HTTP client.
- `New(base, timeout)` — конструктор.
- `envelope` — служебная V5 response envelope.
- `(*Client).get(...)` — общий GET/decode/retCode handler.
- `(*Client).Instruments(ctx)` — активные linear USDT perpetuals и instrument specs.
- `(*Client).Tickers(ctx)` — массовые linear tickers.
- `(*Client).Klines(ctx, symbol, interval, limit)` — свечи в хронологическом порядке.
- `num(string)` — безопасный parser чисел Bybit.

### `internal/universe/universe.go`
Дешёвый первый фильтр рынка.
- `Select(instruments, tickers, minTurnover)` — оставляет торгуемые инструменты с ценой и достаточным 24h turnover, сортирует по ликвидности.

### `internal/indicator/indicator.go`
Атомарные математические функции; не знают о LONG/SHORT.
- `SMA(values, n)` — simple moving average.
- `ATR(high, low, close, n)` — average true range.
- `EfficiencyRatio(close, n)` — отношение направленного displacement к суммарному шуму.
- `DMIADX(high, low, close, n)` — +DI, -DI и ADX.
- `min(a, b)` — локальный integer helper.

### `internal/structure/structure.go`
Структура swing highs/lows.
- `Classify(candles, wing)` — классифицирует последние подтверждённые swings как HH+HL, LH+LL, MIXED или UNKNOWN; ambiguous outside swing не используется как чистый сигнал.

### `internal/features/features.go`
Общее аналитическое ядро.
- `ExtractTimeframe(tf, candles)` — ATR%, DMI/ADX, efficiency, center/drift, momentum, volume ratio, structure и equilibrium features.
- `applyEquilibriumFeatures(features, high, low, close)` — rolling midpoint/range migration и mean-reversion statistics.
- `bounds(high, low)` — high/low заданного rolling-окна.
- `meanReversion(close, midpoint, width)` — midpoint crossings и доля завершённых возвратов после значимых excursions.
- `ClassifyMarket(snapshot, config)` — формирует Direction/Strength/Dynamics/MTF без aggregate score.
- `count([]bool)` — helper подсчёта подтверждений одного типа.
- `min(a, b)` — helper окна.

### `internal/marketdata/service.go`
Связывает Bybit data layer и feature engine.
- `BuildSnapshot(ctx, client, instrument, ticker, config)` — получает 15m/1h/4h candles, строит TF features и итоговый MarketSnapshot.

### `internal/screening/screening.go`
Иерархические strategy gates.
- `Futures(snapshot, direction, config)` — hard gates и evidence для FUTURES LONG/SHORT.
- `Grid(snapshot, direction, instrument, config)` — сначала hard gates equilibrium stability (rolling drift/range expansion/mean reversion), затем capital suitability и LONG_GRID/SHORT_GRID candidate.
- `gridCapitalSuitable(price, instrument, config)` — проверяет, помещается ли минимально полезное число grid orders в reference capital.
- `base(snapshot)` — создаёт общий Candidate и переносит explainability/risk context.

### `internal/output/output.go`
Файловый adapter.
- `Write(dir, name, mode, direction, candidates)` — сериализует стабильный JSON и создаёт файл даже при 0 кандидатов.

## Тестовые файлы

### `internal/config/config_test.go`
- `TestLoadValidation` — не допускает заведомо недостаточную историю.

### `internal/bybit/client_test.go`
- `TestTickers` — проверяет HTTP parsing через `httptest`, без live API.

### `internal/indicator/indicator_test.go`
- `TestEfficiencyRatioTrend` — ER прямого тренда равен 1.
- `TestSMA` — базовая корректность SMA.

### `internal/structure/structure_test.go`
- `TestInsufficientStructure` — недостаточная история даёт UNKNOWN.

### `internal/universe/universe_test.go`
- `TestSelectLiquidity` — низколиквидный инструмент отбрасывается.

### `internal/screening/screening_test.go`
- `cfg()` — тестовая конфигурация.
- `TestFuturesRejectConflict` — MTF conflict невозможно компенсировать другими плюсами.
- `TestGridRejectStrongAcceleration` — сильный accelerating regime является hard block для Grid.
- `TestGridRejectRollingMidpointDrift` — drifting equilibrium нельзя компенсировать правильным LONG/SHORT направлением.
- `TestGridRejectRangeExpansion` — опасное расширение rolling range блокирует Grid.
- `TestGridRejectPoorMeanReversion` — колебания без достаточных возвратов к midpoint не считаются хорошим Grid regime.
- `TestGridAcceptStationaryDirectionalRange` — положительный stationary-range сценарий защищает от модели «всегда NO_GRID».

### `internal/features/features_test.go`
- `candlesRange(...)` — deterministic генератор синтетического range для тестов.
- `TestEquilibriumStationaryRange` — stationary range имеет малый midpoint drift и повторные возвраты.
- `TestEquilibriumDetectsMigratingRange` — движущийся range распознаётся как drifting.

## Что v1 намеренно не делает

Текущая версия: **v1.1.0**. Первое исправление после live-валидации: ADAUSDT была ошибочно пропущена в LONG-GRID при Deep Analyzer `TRENDING_EXPANSION/drifting`. Поэтому Grid hard gates теперь анализируют rolling equilibrium, а не только локальный center drift.

Нет Neutral Grid, БД, execution, Virtual Trader, автоматического leverage, grid range/SL/TP, полноценного WAIT/TRIGGER engine, historical replay и тяжёлой микроструктуры. Это сознательно ограниченный первый проект: сначала требуется проверить качество направления/силы и первичного отбора.

## Разработка

Перед изменениями прочитайте `AGENTS.md`. Ключевое правило: новый индикатор сначала становится атомарным feature и только затем получает явно определённую роль в evidence hierarchy. Не добавляйте total score и не ослабляйте критерии ради количества результатов.
