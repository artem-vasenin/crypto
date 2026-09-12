# STRUCTURE.md — структура проекта Universal Bybit Screener

#
# 0. Документация проекта

## `README.md`

**Назначение:** краткая инструкция по проекту:
- назначение screener и bot;
- список стратегий;
- команды сборки;
- команды тестирования;
- команды запуска screener;
- команды запуска bot;
- описание основных risk settings;
- предупреждение о Testnet/mainnet.

Функций нет.

## `AUDIT.md`

**Назначение:** подробный аудит исходного проекта и список исправлений.

Документ содержит:
- CRITICAL-проблемы;
- HIGH-проблемы;
- MEDIUM-проблемы;
- LOW-проблемы;
- описание конкретных ошибок исходного execution/screener кода;
- что именно исправлено;
- ограничения проведённого тестирования.

Функций нет.

## `STRUCTURE.md`

**Назначение:** карта проекта. Документ описывает назначение каждого файла и каждой функции, а также поток данных и ключевые архитектурные решения.

Функций нет.


# 1. Назначение проекта

Проект состоит из двух разных подсистем:

1. **Screener** — получает рыночные данные Bybit, отбирает ликвидные USDT Linear Perpetual-контракты, рассчитывает индикаторы/структуру/уровни/деривативные метрики и ранжирует монеты по выбранной стратегии.
2. **Bot / Execution Engine** — читает JSON-снимок скриннера, повторно проверяет актуальные рыночные данные перед выставлением заявки и может выставлять PostOnly-лимитные заявки на Bybit. После исполнения контролирует позицию, TP/SL и trailing stop.

Скриннер и торговый бот намеренно разделены через JSON-файл. Это позволяет остановить торговую часть, продолжая исследовать работу скриннера.

---

## 2. Поток данных

```text
Bybit REST
   │
   ├── instruments-info
   ├── tickers
   ├── kline
   ├── open-interest
   └── funding history
   │
   ▼
internal/analysis
   │
   ├── первичный liquidity/price/spread фильтр
   ├── закрытые свечи
   ├── RSI / ATR / volume
   ├── HH/HL/LH/LL
   ├── support/resistance
   ├── OI / funding
   └── order book
   │
   ▼
internal/strategies
   │
   └── score + status + reason
   │
   ▼
strategy-screening.json
   │
   ▼
cmd/bot
   │
   ├── повторная проверка score
   ├── актуальный bid/ask
   ├── instrument limits
   ├── margin/position limits
   ├── SL/TP pre-check
   └── PostOnly order
   │
   ▼
Bybit
   │
   ├── private order WS
   ├── private position WS
   ├── private execution WS
   └── private wallet WS
```

---

# 3. Файлы проекта

## `go.mod`

**Назначение:** описание Go-модуля и его внешних зависимостей.

Содержит:
- имя модуля `universal-bybit-screener`;
- требуемую версию Go `1.26.5`;
- `github.com/gorilla/websocket` для WebSocket;
- `github.com/joho/godotenv` для загрузки `.env`.

Функций нет.

---

## `go.sum`

**Назначение:** контрольные суммы зависимостей Go.

Функций нет.

---

## `Makefile`

**Назначение:** команды сборки и деплоя.

### `build`
Собирает два бинарника:
- `bot`;
- `screener`.

Сначала создаются временные `.tmp`-файлы, чтобы не заменять рабочий бинарник до успешной компиляции.

### `deploy`
- получает изменения из Git;
- запускает сборку;
- останавливает systemd-сервисы;
- заменяет бинарники;
- запускает сервисы.

### `status`
Показывает состояние systemd-сервисов.

---

## `.gitignore`

**Назначение:** исключает из Git секреты, локальные конфиги, бинарники, логи и временные файлы.

Особенно важные исключения:
- `.env`;
- `bot`;
- `screener`;
- `*.tmp`;
- screening JSON;
- логи;
- `.DS_Store`.

---

## `.env.example`

**Назначение:** пример переменных окружения.

Основные переменные:
- `BYBIT_API_KEY`;
- `BYBIT_API_SECRET`.

В исходном проекте также присутствовали переменные Groq. Они не используются текущим screener/bot-кодом.

---

## `configs/config.json`

**Назначение:** основной runtime-конфиг.

### `bybit`
- URL REST API.

### `filters`
- `max_price` — верхняя граница цены монеты;
- `min_turnover_24h` — минимальный 24h turnover;
- `preselect_candidates` — размер предварительной выборки;
- `top_candidates` — количество результатов;
- `max_grid_spread_pct` — максимальный bid/ask spread.

### `analysis`
- количество свечей каждого таймфрейма;
- лимит OI;
- лимит funding;
- глубина order book;
- допустимый возраст order book.

### `execution`
- testnet/mainnet;
- максимальное плечо;
- маржа на сделку;
- общий лимит маржи;
- максимум активных позиций;
- минимальный score;
- trailing stop;
- период проверки;
- timeout зависшей заявки;
- консервативные maker/taker fee;
- дополнительный запас на торговые издержки;
- максимальный процент SL;
- минимальный net-profit после оценочных расходов.

---

# `config/config.go`

**Назначение:** загрузка и нормализация конфигурации.

### `Load`
Читает JSON, десериализует его в промежуточную структуру, преобразует duration-строки в `time.Duration`, устанавливает безопасные значения по умолчанию и возвращает готовый `Config`.

---

# `models/market.go`

**Назначение:** модели рыночных данных и результата скрининга.

Основные структуры:
- `Instrument` — спецификация торгового инструмента Bybit;
- `Ticker` — текущий market snapshot;
- `Candle` — свеча;
- `OpenInterestPoint` — исторический OI;
- `FundingPoint` — исторический funding;
- `Indicators` — рассчитанные индикаторы;
- `Pivot` — pivot high/low;
- `Structure` — состояние HH/HL/LH/LL;
- `Levels` — support/resistance и характеристики диапазона;
- `Derivatives` — funding/OI/spread;
- `OrderBookMetrics` — агрегаты стакана;
- `StrategyResult` — результат конкретной стратегии;
- `Candidate` — полный снимок одной монеты;
- `ScreeningResult` — итоговый JSON скриннера.

Функций нет.

---

# `models/execution.go`

**Назначение:** модели торгового движка.

### `BotConfig`
Хранит настройки исполнения, лимиты риска, комиссии и параметры trailing/pending orders.

### `PositionState`
Хранит локальное состояние позиции или pending order:
- символ;
- сторона;
- order ID;
- entry;
- размер;
- зарезервированная маржа;
- фактическое leverage позиции;
- принадлежность позиции текущему экземпляру (`Managed`);
- SL/TP;
- extrema для trailing;
- состояние pending;
- признак установленного риска.

Функций нет.

---

# `output/json.go`

**Назначение:** безопасная запись screening JSON.

### `WriteJSON`
Сериализует `ScreeningResult` и атомарно записывает его в файл.

### `WriteJSONIfChanged`
Сериализует JSON, вычисляет hash и пропускает запись, если содержимое не изменилось.

### `atomicWrite`
Пишет во временный файл и затем делает `rename`, чтобы потребитель не увидел частично записанный JSON.

---

# `internal/bybit/client.go`

**Назначение:** общий REST GET-клиент Bybit для публичных market endpoints.

### `NewClient`
Создаёт HTTP-клиент с timeout и параметрами retry.

### `doGET`
Выполняет GET-запрос, проверяет HTTP status и Bybit `retCode`, декодирует `result`, выполняет повторные попытки для временных ошибок.

---

# `internal/bybit/helpers.go`

**Назначение:** маленькие вспомогательные функции REST-клиента.

### `f`
Безопасно преобразует строку в `float64`.

### `ts`
Преобразует timestamp в миллисекундах в UTC `time.Time`.

### `fmtInt`
Преобразует положительный integer в строку для query-параметра.

---

# `internal/bybit/instruments.go`

**Назначение:** загрузка списка доступных USDT Linear Perpetual инструментов.

### `Instruments`
Постранично получает `instruments-info`, оставляет только:
- `LinearPerpetual`;
- `USDT` quote;
- `USDT` settle;
- `Trading`.

Также сохраняет:
- tick size;
- min quantity;
- quantity step;
- min notional;
- max order quantity;
- funding interval.

---

# `internal/bybit/tickers.go`

**Назначение:** получение текущего market snapshot.

### `Tickers`
Получает тикеры всех Linear-контрактов и преобразует:
- last price;
- mark/index price;
- 24h change;
- turnover;
- volume;
- bid/ask;
- funding;
- open interest.

---

# `internal/bybit/klines.go`

**Назначение:** REST-загрузка исторических свечей.

### `Klines`
Получает свечи указанного символа и таймфрейма, преобразует их в `models.Candle` и сортирует по времени ASC.

---

# `internal/bybit/derivatives.go`

**Назначение:** деривативные исторические данные.

### `OpenInterest`
Получает исторический OI и сортирует его от старого к новому.

### `Funding`
Получает историю funding rate и сортирует её по времени.

---

# `internal/bybit/ws_klines.go`

**Назначение:** локальный cache свечей, который первоначально прогревается REST и затем обновляется WebSocket.

### `NewKlineCache`
Создаёт cache.

### `Warmup`
Записывает REST-историю конкретного символа и таймфрейма.

### `UpdateWS`
Заменяет текущую свечу при обновлении или добавляет новую свечу.

### `Get`
Возвращает копию массива свечей, защищая внутренний cache от изменения вызывающим кодом.

### `ParseWSKline`
Преобразует WebSocket JSON в `models.Candle`.

### `parseFloat`
Преобразует динамическое JSON-значение в число.

### `fmtString`
Преобразует динамическое значение в строку.

---

# `internal/bybit/ws_orderbook.go`

**Назначение:** локальный order book cache.

### `NewOrderBookCache`
Создаёт cache стаканов.

### `Update`
Применяет snapshot/delta к локальным bid/ask уровням и фиксирует время последнего обновления.

### `GetMetrics`
Берёт лучшие 10 уровней с каждой стороны и рассчитывает:
- bid notional;
- ask notional;
- imbalance;
- bid/ask ratio.

### `Age`
Возвращает возраст последнего обновления стакана. Используется как предохранитель от анализа устаревшего order book.

---

# `internal/bybit/ws_public.go`

**Назначение:** публичный WebSocket Bybit для order book и klines.

### `NewPublicWSStream`
Создаёт поток с cache стакана/свечей и выбранной глубиной order book.

### `topicsForSymbol`
Формирует список подписок одного символа.

### `AddSymbols`
Добавляет новые символы в подписки. Это исправляет исходную проблему, когда после первого screening cycle новые монеты никогда не попадали в WS.

### `Start`
Запускает поток, если он ещё не был запущен.

### `loop`
Подключается к Bybit, подписывается, читает сообщения и автоматически переподключается.

### `sleepContext`
Ожидает заданное время с возможностью немедленной отмены через context.

### `subscribeAllLocked`
Подписывает соединение на все накопленные topics пакетами.

### `keepAlive`
Отправляет WebSocket ping.

### `parseMessage`
Распределяет сообщения между order book cache и kline cache.

---

# `internal/analysis/service.go`

**Назначение:** центральный движок скрининга.

### `NewService`
Создаёт analysis service и все необходимые cache/WS-компоненты.

### `Run`
Полный цикл скрининга:
1. получает instruments;
2. получает tickers;
3. применяет первичные фильтры;
4. сортирует по turnover;
5. выбирает preselection;
6. обеспечивает актуальные свечи и стаканы;
7. параллельно анализирует монеты;
8. сортирует по выбранной стратегии;
9. возвращает Top N.

### `ensureMarketData`
Гарантирует:
- WS запущен;
- новые символы подписаны;
- для новых символов есть REST warmup;
- order book получил свежий snapshot.

### `hasEnoughKlines`
Проверяет наличие достаточного количества свечей в cache.

### `allOrderBooksReady`
Проверяет свежесть стаканов всех выбранных символов.

### `warmupKlinesREST`
Параллельно загружает историю свечей для новых/непрогретых символов.

### `fetchWarmup`
Загружает один конкретный таймфрейм и помещает его в cache.

### `analyze`
Формирует полный `Candidate`:
- только закрытые свечи;
- RSI;
- ATR;
- volume;
- structure;
- support/resistance;
- funding;
- OI;
- spread;
- order book;
- результаты всех стратегий.

Критические производные данные без подтверждения теперь приводят к пропуску кандидата, а не к подстановке нуля.

### `closedCandles`
Убирает последнюю формирующуюся свечу перед расчётом стратегии.

### `fundingAverage24h`
Рассчитывает средний funding за последние 24 часа относительно последней доступной точки.

### `changeN`
Рассчитывает изменение цены за заданное число свечей.

### `spreadPct`
Рассчитывает bid/ask spread.

### `BuildAIPrompt`
Формирует короткое описание назначения screening JSON для последующего AI-анализа.

### `minInt`
Возвращает меньшее из двух целых.

### `maxInt`
Возвращает большее из двух целых.

---

# `internal/indicators/indicators.go`

**Назначение:** математические индикаторы.

### `RSI`
Рассчитывает RSI по Wilder smoothing.

### `ATR`
Рассчитывает ATR по Wilder smoothing.

### `trueRange`
Рассчитывает True Range одной свечи.

### `VolumeRatio`
Сравнивает turnover последней закрытой свечи со средним turnover предыдущих свечей.

В исправленной версии устранён off-by-one исходного алгоритма.

### `VolumeTrend`
Сравнивает средний turnover короткого и длинного периода.

---

# `internal/structure/structure.go`

**Назначение:** анализ рыночной структуры и уровней.

### `Analyze`
Ищет pivot highs/lows и определяет:
- HH;
- LH;
- HL;
- LL;
- EQ.

### `fetchHighState`
Определяет состояние последовательности high.

### `fetchLowState`
Определяет состояние последовательности low.

### `Levels`
Находит ближайшие support/resistance и рассчитывает положение цены внутри диапазона.

### `ApplyATR`
Рассчитывает ширину диапазона в единицах ATR.

---

# `internal/strategies/strategy.go`

**Назначение:** общий интерфейс стратегий и регистрация стратегий.

### `Names`
Возвращает все зарегистрированные стратегии:
- `long`;
- `short`;
- `long-grid`;
- `short-grid`;
- `neutral-grid`.

### `New`
Создаёт стратегию по имени.

В исходной версии grid-стратегии существовали как файлы, но фактически не были зарегистрированы.

### `clamp`
Ограничивает score диапазоном 0–100 и округляет.

### `status`
Преобразует score в статус:
- consider;
- watch;
- risky;
- avoid.

---

# `internal/strategies/long.go`

**Назначение:** directional long screening.

### `Name`
Возвращает `long`.

### `Evaluate`
Применяет:
- защиту от macro downtrend;
- защиту от falling knife;
- volatility gate;
- spread gate;
- order book imbalance;
- funding gate;
- расстояние до сопротивления;
- RSI;
- OI expansion;
- затем считает score по нескольким таймфреймам.

---

# `internal/strategies/short.go`

**Назначение:** directional short screening.

### `Name`
Возвращает `short`.

### `Evaluate`
Зеркально long-стратегии:
- защищает от macro uptrend;
- блокирует активный micro pump;
- проверяет volatility;
- spread;
- ask-side imbalance;
- funding;
- расстояние до поддержки;
- RSI;
- OI;
- затем считает score.

---

# `internal/strategies/long_grid.go`

**Назначение:** поиск кандидатов для long-grid сценария.

### `Name`
Возвращает `long-grid`.

### `Evaluate`
Проверяет наличие диапазона, положение цены, структуру 1h и отношение диапазона к ATR.

Важно: это **не реализация торгового grid-бота**. Это только screening strategy.

---

# `internal/strategies/short_grid.go`

**Назначение:** поиск кандидатов для short-grid сценария.

### `Name`
Возвращает `short-grid`.

### `Evaluate`
Проверяет наличие диапазона, положение цены ближе к resistance, структуру 1h и Range/ATR.

Это также screening strategy, а не исполнение grid-ордера.

---

# `internal/strategies/neutral_grid.go`

**Назначение:** поиск настоящего низковолатильного боковика.

### `Name`
Возвращает `neutral-grid`.

### `Evaluate`
Блокирует:
- направленный 1h trend;
- направленный 4h trend;
- broadening formation HH+LL;
- contracting formation LH+HL;
- отсутствие диапазона;
- слишком высокую ATR 1h;
- слишком узкий диапазон;
- слишком широкий диапазон;
- положение цены возле границы.

Исходная версия была слишком мягкой и могла пропускать волатильные/направленные ситуации.

---

# `internal/execution/risk.go`

**Назначение:** расчёт размера позиции и защитных уровней.

### `CalculateDynamicStopLoss`
Строит SL относительно pivot и ATR. Если pivot отсутствует, используется только ATR-дистанция.

### `CalculateDynamicTakeProfit`
Рассчитывает TP по минимальному R:R.

### `CalculateDynamicLeverage`
Снижает плечо при высокой ATR.

### `CalculatePositionQty`
Рассчитывает количество контрактов и **не увеличивает молча размер позиции**, если минимальный notional биржи больше допустимого размера сделки.

### `RoundToStep`
Округляет значение к шагу.

### `floorToStep`
Округляет вниз.

### `roundPriceDown`
Округляет цену вниз.

### `roundPriceUp`
Округляет цену вверх.

### `GetPrecision`
Определяет количество знаков после запятой для шага.

### `FormatStep`
Форматирует значение для API Bybit.

### `ValidateStopLoss`
Проверяет направление SL, минимальную ATR-дистанцию и максимальный риск.

### `ValidateTakeProfit`
Проверяет направление TP и минимальную дистанцию.

### `EstimatedRoundTripCostPct`
Оценивает round-trip trading cost с запасом.

---

# `internal/execution/engine.go`

**Назначение:** основной торговый движок.

### `NewEngine`
Создаёт execution engine, локальное состояние и WebSocket engine.

### `InitWebSocket`
Запускает публичный и приватный Bybit WebSocket.

### `RefreshState`
При старте синхронизирует:
- balance;
- открытые позиции;
- открытые заявки.

Это исправляет проблему исходного бота, который после рестарта практически не знал реальное состояние счёта.

### `handleBalanceUpdateWS`
Обновляет локальный доступный баланс.

### `handlePositionUpdateWS`
Синхронизирует локальную позицию с реальным position stream.

Размер позиции теперь является источником истины вместо предположения, что отправленная заявка уже стала позицией.

### `handleOrderUpdateWS`
Отслеживает lifecycle заявки и удаляет pending-состояние после отмены/reject.

### `handleExecutionWS`
Обрабатывает фактические execution events:
- удаляет дубликаты по `ExecID`;
- игнорирует execution чужих/неуправляемых позиций;
- проверяет `OrderID`;
- проверяет соответствие стороны (`long=Buy`, `short=Sell`);
- сохраняет entry snapshot только после фактического fill;
- запускает fallback-установку SL/TP.

### `attachRiskAfterFill`
Fallback для установки SL/TP после фактического fill, если они не были подтверждены/установлены.

### `calculateRiskLevels`
Единая проверка SL/TP до отправки заявки.

### `RefreshBalance`
Получает `totalAvailableBalance` Unified Account.

### `ProcessCandidate`
Основная торговая функция. Созданная здесь позиция получает `Managed=true`, поэтому только этот экземпляр execution engine имеет право обрабатывать её execution и trailing lifecycle.
- проверяет стратегию;
- проверяет cooldown;
- проверяет число позиций;
- проверяет общий лимит маржи;
- проверяет баланс;
- получает свежий bid/ask;
- получает instrument limits;
- рассчитывает quantity;
- устанавливает leverage;
- рассчитывает SL/TP;
- выставляет PostOnly order на maker-стороне spread.

### `sideEntryPrice`
Возвращает bid для Buy и ask для Sell.

### `FormatPriceForSide`
Округляет Buy вниз и Sell вверх, чтобы не превратить maker-заявку в taker.

### `CleanupPendingOrders`
Отменяет слишком долго висящие pending orders.

### `UpdateTrailingStops`
Передвигает SL только в сторону уменьшения риска.

### `LogActivePositions`
Выводит локальное состояние позиций.

### `SetTradingStopMarkPrice`
Устанавливает full-position TP/SL через Bybit `trading-stop`.

### `buildPostOnlyOrderParams`
Строит и форматирует параметры Bybit для открытия позиции. Проверяет тестами, что сторона, PostOnly, TP/SL и Full TP/SL mode передаются согласованно.

### `placePostOnlyOrder`
Выставляет Limit/PostOnly заявку. TP/SL передаются уже при создании заявки с `tpslMode=Full`, `MarkPrice` и рыночным исполнением TP/SL. После фактического fill движок дополнительно выполняет `trading-stop`, пересчитывая уровни от реальной цены исполнения.

### `cancelOrder`
Отменяет зависшую заявку.

### `setLeverage`
Устанавливает leverage и допускает только ожидаемый ответ "leverage unchanged".

### `refreshPositions`
Получает реальные открытые позиции после старта. Позиции, происхождение которых текущий процесс не может доказать, помечаются как `Managed=false`. Это предотвращает управление чужой позицией при одновременном запуске long/short ботов.

### `refreshOpenOrders`
Получает реальные открытые заявки после старта и не выдаёт им статус управляемых заявок текущего процесса.

### `getLiveTicker`
Получает актуальные bid/ask непосредственно перед торговой заявкой.

### `getInstrumentLimits`
Получает tick size, quantity step, min/max quantity и min notional.

### `publicGET`
Выполняет публичный GET и проверяет Bybit envelope.

### `doSignedGET`
Выполняет подписанный GET Bybit V5.

### `doSignedPOST`
Выполняет подписанный POST Bybit V5.

### `checkAPIRetCode`
Проверяет `retCode` ответа Bybit.

### `parseFloat`
Преобразует строку API в `float64`.

### `mathCeilToStep`
Округляет значение вверх к торговому шагу.

### `mathAbsPct`
Считает процентную дистанцию.

---

# `internal/execution/ws_engine.go`

**Назначение:** приватный и публичный WebSocket торгового движка.

### `NewWSEngine`
Создаёт WebSocket engine и callback-и.

### `GetLatestPrice`
Возвращает последнюю известную цену.

### `GetBTCTrend15m`
Возвращает изменение BTC относительно 15-минутной базовой цены.

### `StartPublicTickerStream`
Запускает публичный ticker stream.

### `connectAndReadPublic`
Подключается к публичному stream и восстанавливает соединение.

### `parsePublicMessage`
Извлекает last price из ticker сообщений.

### `SubscribeTicker`
Добавляет конкретный символ в ticker subscriptions.

### `resubscribeTickersLocked`
Повторно подписывает ticker topics после reconnect.

### `StartPrivateStream`
Запускает private stream.

### `connectAndReadPrivate`
Авторизуется и подписывается на:
- position;
- wallet;
- execution;
- order.

### `startHeartbeat`
Отправляет heartbeat ping.

### `parsePrivateMessage`
Разбирает четыре типа private событий и передаёт их в callbacks.

### `sleepContext`
Ожидает reconnect delay с поддержкой context cancellation.

---

# `internal/execution/snapshot_logger.go`

**Назначение:** сохранение торгового снимка при необходимости аудита.

### `SaveTradeSnapshot`
Создаёт JSON-файл только для фактически исполненного входа. В файл попадают:
- символ;
- стороной;
- ценой;
- размером;
- leverage;
- order ID;
- причиной стратегии;
- полным candidate snapshot;
- BTC trend;
- фактические `execution_id`, `execution_time` и комиссия.

Исправлена потенциальная panic при слишком коротком `orderID`. Файл создаётся рядом с исполняемым бинарником в `snapshots/`.

---

# `internal/execution/risk_test.go`

**Назначение:** unit-тесты критической математики риска.

Проверяются:
- корректность SL;
- fallback при отсутствии pivot;
- rejection при недостаточном min notional;
- TP;
- динамическое снижение leverage;
- корректность параметров PostOnly + TP/SL заявки.

---

# `internal/strategies/strategy_test.go`

**Назначение:** unit-тесты регистрации стратегий и neutral-grid.

Проверяется:
- наличие всех пяти стратегий;
- rejection directional trend для neutral-grid.

---

# `cmd/screener/main.go`

**Назначение:** executable для запуска скриннера.

### `main`
- загружает конфиг;
- создаёт выбранную стратегию;
- создаёт Bybit client;
- создаёт analysis service;
- запускает первый screening cycle;
- затем повторяет его по interval.

### `executeScreening`
Запускает один screening cycle с timeout, сохраняет JSON и пишет результат в лог.

---

# `cmd/bot/main.go`

**Назначение:** executable торгового бота.

### `main`
- проверяет, что стратегия `long` или `short`;
- загружает конфиг;
- читает API credentials;
- создаёт execution engine;
- запускает WebSocket;
- синхронизирует состояние счёта;
- периодически обрабатывает screening JSON.

### `processIteration`
- отменяет зависшие pending orders;
- читает screening JSON;
- параллельно обрабатывает кандидатов;
- обновляет trailing stops;
- вызывает `ProcessCandidate`.

---

# `output/json.go`

См. описание выше. Это единственный слой, отвечающий за запись screening snapshot на диск.

---

## 4. Файлы, которые были в исходном ZIP, но не являются исходниками

### `internal/.DS_Store`
macOS metadata-файл. На работу проекта не влияет. В исправленный архив не включён.

### `cmd/.DS_Store`
macOS metadata-файл. На работу проекта не влияет. В исправленный архив не включён.

---

# 5. Что принципиально изменено

1. Зарегистрированы все пять screening strategies.
2. Bot больше не пытается торговать `long-grid`, `short-grid` и `neutral-grid` как обычными directional позициями.
3. PostOnly entry теперь выставляется на правильной стороне spread.
4. TP/SL рассчитываются **до** заявки и передаются вместе с order create.
5. Добавлен pre-trade risk gate.
6. Добавлен реальный max-total-margin контроль.
7. Pending order отделён от фактически открытой позиции.
8. Добавлен order WebSocket lifecycle.
9. Добавлен startup reconciliation с Bybit.
10. Добавлена отмена зависших pending orders.
11. Убран вызов устаревшего `switch-isolated` из торгового пути.
12. Баланс Unified Account берётся через `totalAvailableBalance`.
13. Установлен обязательный `tpslMode=Full` для position trading stop.
14. Скриннер анализирует закрытые свечи.
15. Исправлен VolumeRatio off-by-one.
16. WS universe теперь обновляется при изменении Top-N.
17. Order book имеет проверку свежести.
18. Neutral-grid сделан существенно строже.
19. Quantity больше не увеличивается молча сверх допустимой маржи ради exchange minimum.
20. Добавлены unit-тесты критической risk/strategy логики.

---

# 6. Проверка исправленной версии

Production-архив сохраняет `go 1.26.5`. В среде подготовки архива Go 1.26.5 был недоступен, поэтому полный набор `go test ./...` дополнительно прогонялся на Go 1.23.2 с локальными compile-only заглушками только для `gorilla/websocket` и `godotenv`. Все пакеты и unit-тесты прошли. Заглушки в production-архив не входят.

Дополнительно проверено статически:
- `PositionState` содержит `Leverage`, который используется execution engine;
- `short` всегда соответствует `Sell`, `long` — `Buy`;
- PostOnly для Buy ставится по bid, для Sell — по ask;
- TP/SL передаются в `/v5/order/create`;
- Full TP/SL использует market trigger orders;
- snapshot создаётся только после execution event и защищён от повторной записи;
- execution с неправильной стороной блокирует символ;
- Unified Account balance читается через `totalAvailableBalance`.

# 7. Важное ограничение

Это исправленная и существенно более безопасная версия исходного проекта, но это всё ещё не production-grade торговая система.

Перед реальной торговлей необходимо отдельно проверить на Bybit Testnet:
- one-way position mode;
- фактические fee rates аккаунта;
- поведение PostOnly;
- partial fills;
- TP/SL после partial fill;
- reconnect WebSocket;
- отмену pending orders;
- restart/reconciliation;
- liquidation/risk-limit behaviour;
- rate limits;
- реальные min/max order quantities.

Особенно важно не считать наличие unit-тестов доказательством корректности реальной торговли: unit-тесты не могут проверить внешний matching engine Bybit.

# 8. Дополнение после практической проверки execution

## `internal/execution/snapshot_logger.go` — уточнение

`SaveTradeSnapshot` вызывается из execution lifecycle только после подтверждённого события `execution` от Bybit. Создание PostOnly-заявки само по себе snapshot не создаёт.

Сохраняемые дополнительные данные:
- фактическая комиссия исполнения;
- `execution_id`;
- фактическое время исполнения;
- фактические цена и количество исполнения;
- order ID;
- leverage;
- полный screening `Candidate`.

## `internal/execution/ws_engine.go` — уточнение

`ExecutionLog` теперь также содержит `OrderID`, полученный из private execution stream. Это позволяет однозначно связать фактический fill с заявкой, которая его породила.

## `internal/execution/engine.go` — уточнение

`handleExecutionWS` выполняет следующие важные действия:

1. Отбрасывает дубликаты execution по `ExecID`.
2. Проверяет, что execution относится к управляемой этим процессом заявке.
3. Проверяет `OrderID` и сторону фактического исполнения.
4. При первом фактическом входе сохраняет immutable entry snapshot и запускает установку SL/TP.

При несовпадении направления (`short`/`Sell` ожидает `Buy` и наоборот) символ помещается в `disabledTokens`, а в лог пишется критическая ошибка.

## `models/execution.go` — уточнение

`PositionState` дополнительно хранит:

- `Leverage` — leverage конкретной позиции;
- `SnapshotSaved` — защита от повторного создания entry snapshot при нескольких execution events, например при partial fills;
- `Managed` — признак, что позиция/заявка была создана текущим экземпляром execution engine.
