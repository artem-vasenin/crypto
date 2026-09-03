# Архитектура Universal Bybit Screener & Trading Bot

Высокопроизводительный торговый комплекс на Go для автоматизированного скрининга и исполнения сделок на Bybit V5 Unified Margin API.

## Структура директорий и файлов

### Демоны и Точки Входа (`/cmd`)
* `cmd/bot/main.go` — Сервис автоматического исполнения сделок. Читает JSON-результат скринера, проверяет баланс, лимиты риска и открывает/сопровождает позиции по стратегии.
* `cmd/screener/main.go` — Сервис-скринер. Постоянный демон, который асинхронно анализирует весь рынок Bybit USDT Linear, рассчитывает индикаторы и формирует `screening.json`.

### Конфигурация (`/config`, `/configs`)
* `config/config.go` — Модуль загрузки и парсинга `configs/config.json`. Переводит тайм-ауты в `time.Duration` и валидирует границы параметров.
* `configs/config.json` — Единый конфигурационный файл настроек фильтрации, лимитов капитала, плечей и параметров подключения.

### Взаимодействие с Bybit API (`/internal/bybit`)
* `internal/bybit/client.go` — Базовый REST-клиент. Выполняет HTTP GET с автоматической обработкой ошибок 429/500, экспоненциальным бэкоффом (Retries) и расшифровкой Bybit Envelope.
* `internal/bybit/instruments.go` — Загрузка контрактов (`Instruments`). Оценивает минимальный шаг лота (`QtyStep`), шаг цены (`TickSize`) и Notional Value.
* `internal/bybit/tickers.go` — Вычитка 24h среза рынка (цены, суточные обороты, спреды, фандинг, открытый интерес).
* `internal/bybit/klines.go` — REST-метод запроса исторических свечей `Klines` для первичного разового прогрева ОЗУ при старте.
* `internal/bybit/derivatives.go` — Метод запроса последовательности Open Interest и исторического Funding Rate.
* `internal/bybit/ws_public.go` — **[NEW]** Менеджер дуплексного публичного WebSocket. Поддерживает стримы L2 OrderBook и Klines (15m, 1h, 4h) с Heartbeat (20s) и авто-реконнектом.
* `internal/bybit/ws_klines.go` — **[NEW]** Потокобезопасный кольцевой кэш свечей в памяти (`KlineCache`). Позволяет вычислять индикаторы за O(1) без REST-запросов.
* `internal/bybit/helpers.go` — Парсеры типов строк Bybit в `float64` и `time.Time`.

### Торговый Движок и Риски (`/internal/execution`)
* `internal/execution/engine.go` — Исполнительный модуль. Логика отправки ордеров, мониторинг Trailing Stop, контроль PnL и ведение стейта активных позиций.
* `internal/execution/risk.go` — Модуль математического расчета риска. Динамическое плечо (`CalculateDynamicLeverage`), точный объем лота (`CalculatePositionQty`) и валидаторы SL/TP.
* `internal/execution/ws_engine.go` — Приватный WebSocket-клиент бота. Выполняет авторизацию HMAC-SHA256, слушает каналы `position`, `wallet`, `execution` и мгновенно фиксирует PnL закрытых сделок.

### Аналитика, Индикаторы и Структура (`/internal/analysis`, `/internal/indicators`, `/internal/structure`)
* `internal/analysis/service.go` — Главный оркестратор скрининга. Соединяет данные рынка, индикаторы и прогоняет вектор каждой монеты через стратегии.
* `internal/indicators/indicators.go` — Математический расчет Wilder RSI, ATR, VolumeRatio (по USDT Turnover) и VolumeTrend.
* `internal/structure/structure.go` — Алгоритм обнаружения фрактальных Pivot High / Pivot Low, определения бычьей/медвежьей структуры (HH, HL, LH, LL) и параметров диапазона (`RangePositionPct`).

### Стратегии (`/internal/strategies`)
* `internal/strategies/strategy.go` — Интерфейс `Strategy` и фабричный метод `New(name)`.
* `internal/strategies/long.go` — Эвристический модуль Long. Учитывает приток OI, L2 Imbalance, RSI и Hard Gates фильтры по верху диапазона.
* `internal/strategies/short.go` — Эвристический модуль Short. Учитывает продажи OI, давление Ask в L2 стакане и Hard Gates по низу диапазона.
* `internal/strategies/long_grid.go`, `short_grid.go`, `neutral_grid.go` — Эвристические оценки каналов для Grid-алгоритмов.

### Модели и Форматирование (`/models`, `/output`)
* `models/market.go` — Go-структуры рыночных данных, свечей, индикаторов, уровня и стакана.
* `models/execution.go` — Структуры конфигурации бота и состояния позиций.
* `output/json.go` — Атомарная запись результатов скринера на диск через `.tmp`-замену и проверка MD5-хэша от изменений.