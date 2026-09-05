# Crypto Coin Analyzer

CLI-приложение для глубокого анализа **одной монеты Bybit USDT Perpetual**.

Главная идея проекта: вместо скриннинга сотен монет приложение принимает один символ, например `SOLUSDT`, собирает для него технические, рыночные и деривативные данные и формирует большой JSON. Этот JSON можно целиком передать ИИ и попросить определить, есть ли смысл искать `LONG`, `SHORT` или лучше `WAIT`.

Структура JSON намеренно близка к формату твоего Bybit-скринера: `market`, `indicators`, `trend`, `momentum`, `volume`, `structure`, `levels`, `derivatives`, `order_book`, `strategies`. Дополнительно добавлен отдельный блок `btc_context`, потому что для directional Long/Short важно понимать состояние BTC и возможный lead-lag.

## Что получает приложение

Используются публичные REST API Bybit V5, без API key:

- ticker текущего линейного контракта;
- 15m / 1h / 4h свечи;
- до 7 дней 1m свечей для краткосрочного BTC lead-lag;
- BTCUSDT ticker и 1m свечи;
- funding rate и историю funding;
- open interest и изменение OI;
- long/short ratio;
- стакан до 200 уровней.

Bybit документирует `kline`, `tickers`, `orderbook`, `open interest`, funding history и long/short ratio как публичные V5 market endpoints. В частности, kline поддерживает интервалы от 1 минуты до месяца, а orderbook для linear-контрактов — до 1000 уровней в API, хотя приложение намеренно использует 200, чтобы сохранить совместимость с текущей схемой твоего скринера.

## Основные возможности

### 1. Технический анализ

Рассчитываются:

- RSI 15m / 1h / 4h;
- ATR 15m / 1h / 4h;
- ATR в процентах от цены;
- EMA 20 / 50 / 200 на 15m / 1h / 4h;
- отклонение цены от EMA на 1h;
- изменение цены за 1h / 4h / 12h / 24h;
- volume ratio;
- объёмы 5m / 15m / 1h.

### 2. Структура рынка

На 1h истории ищутся простые pivot high / pivot low.

На их основе определяется:

- `HH` / `LH` для максимумов;
- `HL` / `LL` для минимумов;
- последние pivot highs/lows.

Это не пытается изображать из себя полноценный Smart Money индикатор. Цель — дать ИИ исходные факты для проверки структуры.

### 3. Support / Resistance

Определяются ближайшие уровни сопротивления и поддержки на последнем участке 1h истории.

Также рассчитываются:

- ширина диапазона;
- положение текущей цены внутри диапазона;
- отношение диапазона к ATR 1h.

Эти поля особенно полезны для оценки grid-сценариев.

### 4. Деривативы

В JSON попадают:

- текущий funding;
- средний funding по последним значениям;
- Open Interest;
- изменение OI;
- long/short ratio.

Важно: `openInterest` у Bybit имеет единицу измерения, зависящую от типа контракта. Здесь используется `linear`, поэтому значение относится к базовой монете; Bybit также отдаёт `openInterestValue`, но для текущего анализа основной акцент сделан на OI и его изменении.

### 5. Стакан

Рассчитываются:

- суммарный bid notional;
- суммарный ask notional;
- imbalance в процентах;
- bid/ask ratio;
- spread;
- количество использованных уровней.

### 6. BTC context

Это отдельная важная часть проекта.

Приложение сравнивает минутные доходности монеты и BTCUSDT и проверяет несколько лагов:

- 0 минут;
- 1 минута;
- 2 минуты;
- 3 минуты;
- 5 минут;
- 10 минут.

В JSON попадут, например:

```json
"btc_context": {
  "corr_1m_0": 0.71,
  "corr_1m_1": 0.76,
  "corr_1m_2": 0.81,
  "corr_1m_3": 0.84,
  "corr_1m_5": 0.77,
  "corr_1m_10": 0.62,
  "best_lag_minutes": 3,
  "best_lag_correlation": 0.84
}
```

Это не торговый сигнал само по себе. Корреляция показывает статистическую связь на выбранном окне, а не гарантирует, что монета обязательно «догонит» BTC.

## Структура проекта

```text
crypto-coin-analyzer/
├── cmd/
│   └── analyzer/
│       └── main.go
├── internal/
│   ├── analysis/
│   │   ├── build.go
│   │   ├── build_test.go
│   │   └── types.go
│   ├── bybit/
│   │   └── client.go
│   ├── indicators/
│   │   ├── indicators.go
│   │   └── indicators_test.go
│   └── output/
│       └── json.go
├── testdata/
├── .gitignore
├── go.mod
├── Makefile
└── README.md
```

## Описание каждого файла

### `cmd/analyzer/main.go`

Точка входа приложения.

Функции и ответственность:

- разбирает CLI-параметры;
- проверяет символ;
- ограничивает глубину 1m истории диапазоном 1–7 дней;
- создаёт Bybit-клиент;
- вызывает `analysis.Build`;
- выводит JSON в stdout;
- при `-out` дополнительно сохраняет JSON в файл.

Основная функция:

- `main()` — запуск анализа и обработка ошибок.

### `internal/bybit/client.go`

Низкоуровневый публичный клиент Bybit V5.

Основные функции:

- `NewClient()` — создаёт HTTP-клиент;
- `get()` — общий GET-запрос, проверка HTTP и `retCode` Bybit;
- `Ticker()` — текущий ticker;
- `Klines()` — исторические свечи;
- `Funding()` — история funding;
- `OpenInterest()` — история OI;
- `LongShort()` — long/short ratio;
- `OrderBook()` — snapshot стакана.

Здесь специально нет API key/secret: приложение работает только с публичными market endpoints.

### `internal/indicators/indicators.go`

Чистые математические функции индикаторов.

Основные функции:

- `EMA()` — экспоненциальная скользящая средняя;
- `RSI()` — Relative Strength Index;
- `ATR()` — Average True Range;
- `PercentChange()` — процентное изменение;
- `Mean()` — среднее значение;
- `Std()` — стандартное отклонение;
- `Correlation()` — корреляция Пирсона;
- `Returns()` — последовательность доходностей;
- `VolumeRatio()` — отношение среднего короткого объёма к длинному.

Файл не знает ничего о Bybit — поэтому индикаторы легко тестировать отдельно.

### `internal/indicators/indicators_test.go`

Unit-тесты математической части.

Проверяются:

- EMA;
- RSI на растущем ряду;
- Pearson correlation.

### `internal/analysis/types.go`

Все структуры итогового JSON.

Здесь описаны:

- `Report`;
- `Market`;
- `Indicators`;
- `Trend`;
- `Momentum`;
- `Volume`;
- `Structure`;
- `Levels`;
- `Derivatives`;
- `OrderBook`;
- `BTCContext`;
- `Strategies`;
- `AIInstructions`.

Изменяя этот файл, можно расширять формат JSON без изменения CLI.

### `internal/analysis/build.go`

Главный аналитический модуль.

Основные функции:

- `Build()` — собирает данные и формирует полный `Report`;
- `buildStructure()` — определяет pivot highs/lows и HH/LH/HL/LL;
- `buildLevels()` — рассчитывает support/resistance и характеристики диапазона;
- `buildBTCContext()` — рассчитывает BTC lead-lag;
- `alignReturns()` — выравнивает минутные доходности;
- `lagCorr()` — считает корреляцию при выбранном лаге;
- `scoreStrategies()` — даёт вспомогательные scores для Long/Short и grid-стратегий;
- `mk()` — переводит score в статус `avoid/risky/watch/consider`.

`scoreStrategies()` намеренно остаётся вспомогательным. ИИ получает все исходные данные и не должен слепо доверять этому score.

### `internal/analysis/build_test.go`

Интеграционный unit-тест аналитического сборщика с fake Bybit API.

Вместо реального интернета используется `fakeAPI`, поэтому тест:

- быстрый;
- детерминированный;
- не зависит от текущего рынка;
- не требует API key;
- не создаёт нагрузку на Bybit.

### `internal/output/json.go`

Минимальный слой сериализации.

Основная функция:

- `WriteJSON()` — пишет JSON с опциональным pretty formatting.

Отдельный пакет оставлен специально, чтобы позднее можно было добавить другие форматы или HTTP-вывод, не смешивая это с аналитикой.

### `Makefile`

Команды разработки:

```text
make fmt    # gofmt
make vet    # go vet
make test   # форматирование + unit tests
make build  # test + сборка бинарника
make run    # пример запуска SOLUSDT
make clean  # удаление локальных результатов
```

### `.gitignore`

Исключает бинарники, логи, JSON-результаты локальных запусков и файлы IDE/macOS.

### `go.mod`

Описание Go-модуля и версии языка.

Проект написан на стандартной библиотеке Go и не имеет внешних зависимостей. В архиве указана минимальная совместимая версия Go, использованная для проверки; на Go 1.26 проект также должен собираться без изменений.

## Установка

Требуется Go 1.23+.

Проверка:

```bash
go version
```

Клонирование/распаковка проекта и переход в каталог:

```bash
cd crypto-coin-analyzer
```

Проверка проекта:

```bash
go test ./...
```

## Сборка:

Дефолтная ОС
```bash
go build -o crypto-coin-analyzer ./cmd/analyzer
```

Windows
```bash
GOOS=windows GOARCH=amd64 go build -o crypto-coin-analyzer.exe ./cmd/analyzer
```

## Запуск

Самый простой вариант:

```bash
go run ./cmd/analyzer -symbol SOLUSDT
```

JSON будет выведен в stdout.

С сохранением результата:

```bash
go run ./cmd/analyzer -symbol SOLUSDT -out SOLUSDT.json
```

С глубиной 3 дня для минутного BTC lead-lag:

```bash
go run ./cmd/analyzer -symbol SOLUSDT -days 3 -out SOLUSDT.json
```

После сборки:

```bash
./crypto-coin-analyzer -symbol SOLUSDT -out SOLUSDT.json
```

## Какие символы использовать

Примеры:

```text
BTCUSDT
ETHUSDT
SOLUSDT
XRPUSDT
DOGEUSDT
FARTCOINUSDT
```

Нужен именно символ линейного USDT-контракта Bybit.

## Формат результата

Верхний уровень JSON:

```text
Report
├── generated_at
├── exchange
├── category
├── symbol
├── purpose
├── data_quality
├── market
├── indicators
├── trend
├── momentum
├── volume
├── structure
├── levels
├── derivatives
├── order_book
├── btc_context
├── strategies
└── ai_instructions
```

### `market`

Текущая цена, изменения 24h/3d/7d, оборот, объём и spread.

### `indicators`

RSI и ATR на нескольких таймфреймах.

### `trend`

EMA 20/50/200 и расстояние цены до EMA.

### `momentum`

Изменение цены на нескольких горизонтах.

### `volume`

Текущие объёмы и volume ratios.

### `structure`

Pivot levels и HH/LH/HL/LL.

### `levels`

Support/resistance и характеристики диапазона.

### `derivatives`

Funding, OI и long/short ratio.

### `order_book`

Дисбаланс bid/ask и ликвидность в стакане.

### `btc_context`

Состояние BTC, относительная доходность и проверка lead-lag.

### `strategies`

Вспомогательные scores для пяти знакомых стратегий:

- long;
- long-grid;
- neutral-grid;
- short;
- short-grid.

### `ai_instructions`

Готовый контекст для ИИ: что анализировать, какие правила соблюдать и какие поля желательно вернуть.

## Как использовать JSON с ИИ

После запуска:

```bash
go run ./cmd/analyzer -symbol SOLUSDT -out SOLUSDT.json
```

Открываешь `SOLUSDT.json`, копируешь его в ИИ и даёшь запрос примерно такого типа:

```text
Ты — криптоаналитик. Проанализируй JSON одной монеты Bybit.

Определи, есть ли сейчас обоснованный сценарий LONG или SHORT.

Не доверяй strategy score вслепую. Проверь исходные данные.

Отдельно проверь:
- структуру 15m/1h/4h;
- EMA и momentum;
- RSI и ATR;
- объём;
- support/resistance;
- funding;
- Open Interest;
- long/short ratio;
- стакан;
- движение BTC;
- BTC lead-lag;
- противоречия между показателями.

Если преимущества одной стороны нет — дай WAIT.

Для выбранного сценария дай:
1. LONG / SHORT / WAIT.
2. Уверенность 0-100.
3. Зону входа.
4. SL.
5. TP1/TP2/TP3.
6. Основной сценарий.
7. Условие отмены сценария.
8. Главные аргументы ЗА.
9. Главные аргументы ПРОТИВ.
```

## Важное ограничение

Это аналитический сборщик, а не торговый бот.

Он **не открывает позиции**, не хранит API keys и не отправляет ордера.

Особенно важно не интерпретировать BTC lead-lag как гарантированный арбитраж. Корреляция на историческом окне может исчезнуть, измениться при смене режима рынка или быть вызвана общим фактором, а не настоящей причинной задержкой.

## Проверка проекта

Локально выполнено:

```text
go test ./...

?    crypto-coin-analyzer/cmd/analyzer       [no test files]
?    crypto-coin-analyzer/internal/bybit    [no test files]
ok   crypto-coin-analyzer/internal/analysis
?    crypto-coin-analyzer/internal/output    [no test files]
ok   crypto-coin-analyzer/internal/indicators
```

Тесты используют mock/fake API.

В среде сборки отсутствовал исходящий интернет, поэтому реальный запуск бинарника против `api.bybit.com` из sandbox выполнить не удалось. Формат и endpoints сверены с актуальной документацией Bybit V5; live-интеграционный тест нужно выполнить уже на твоём Mac/VPS с интернетом.

## Что можно улучшить во второй версии

Наиболее полезные следующие шаги:

1. Добавить более точный поиск зон поддержки/сопротивления через кластеризацию pivot levels.
2. Добавить реальную оценку divergence RSI/цены.
3. Добавить ATR expansion/contraction.
4. Добавить liquidation data, если выбран подходящий публичный endpoint.
5. Добавить funding trend, а не только среднее.
6. Улучшить BTC lead-lag через rolling windows вместо одного общего окна.
7. Добавить несколько BTC lead-lag окон: 30m, 2h, 6h.
8. Добавить beta относительно BTC.
9. Добавить нормализованную относительную силу монеты относительно BTC.
10. Добавить исторический backtest самого lead-lag сигнала.

Последний пункт особенно важен: именно backtest покажет, является ли обнаруженный lag реальным торговым преимуществом или просто красивой корреляцией.
