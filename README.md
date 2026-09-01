# Production-Ready Bybit Perpetual Crypto Screener

Высокопроизводительный асинхронный скринер и аналитический демон для линейных перпетуальных контрактов **Bybit (USDT Perpetual)** на Go.

Система непрерывно мониторит рыночную структуру, деривативные метрики (Funding Rate, Open Interest), волатильность (ATR) и глубину L2-стакана (через WebSockets) для отбора квалифицированных торговых сетапов по пяти алгоритмическим стратегиям.

## Архитектурные особенности

* **Daemon Architecture (Continuous Screening):** Приложение работает как фоновый сервис в бесконечном цикле (`time.Ticker`), изолируя каждую итерацию анализа с помощью таймаутов контекста (`context.WithTimeout`) и обрабатывая системные сигналы завершения (`SIGINT`/`SIGTERM`).
* **In-Memory L2 OrderBook Cache (WebSockets):** Потокобезопасное кэширование L2-стакана (`sync.RWMutex`) с точечной подпиской на топ-кандидатов. Слияние дельт стакана и расчет `ImbalancePct` / `BidAskRatio` выполняется за $O(1)$ без нагрузки на REST API и риска получения `HTTP 429` (Rate Limits).
* **Price / Open Interest Matrix:** Направленные стратегии (`long`, `short`) валидируют моментум с помощью матрицы дельты цены и открытого интереса. Это исключает ложные сигналы, вызванные каскадными ликвидациями (Long Liquidation) или закрытием коротких позиций (Short Covering).
* **Hard Gates (Контроль рисков):** Автоматическая забраковка кандидатов при обнаружении токсичных формаций: расширяющиеся клины (`HH+LL`), рассинхрон волатильности (ATR к ширине канала), торговлю на ATH без верхних уровней сопротивления.
* **Atomic JSON Storage with MD5 Delta Control:** Запись результатов выполняется атомарно через подмену `.tmp` файлов. Внедрен контроль MD5-хэша полезной нагрузки: файл обновляется на диске только при обнаружении реальных изменений в выборке.

---

## Дерево проекта и описание модулей

```text
universal-bybit-screener/
├── cmd/
│   └── screener/
│       └── main.go                 # Точка входа: инициализация демона, обработка CLI-флагов и сигналов ОС
├── config/
│   └── config.go                   # Строгая типизация конфигурации, парсинг time.Duration и валидация
├── configs/
│   └── config.json                 # Конфигурационный файл (таймауты, лимиты REST/WS, пороги фильтров)
├── internal/
│   ├── analysis/
│   │   └── service.go              # Пайплайн скрининга, оркестрация параллельных REST/WS запросов, генерация Prompt
│   ├── bybit/
│   │   ├── client.go               # Базовый HTTP-клиент с Retry/Backoff и контролем retCode
│   │   ├── funding.go              # Получение истории фандинга и расчет средневзвешенного за 24h
│   │   ├── helpers.go              # Вспомогательные функции конвертации типов и парсинга
│   │   ├── instruments.go          # Загрузка и фильтрация активных USDT Linear Perpetual контрактов
│   │   ├── klines.go               # Загрузка и сортировка свечных данных (15m, 1h, 4h)
│   │   ├── open_interest.go        # История и процентное изменение Open Interest
│   │   ├── tickers.go              # Снимок тикеров 24h (цены, объемы, turnover)
│   │   └── ws_orderbook.go         # WS-клиент, менеджер подписок, L2 OrderBook Cache и расчет стаканного имбаланса
│   ├── indicators/
│   │   └── indicators.go           # Вычисление Wilder RSI, ATR, VolumeRatio1h и VolumeTrend1h
│   ├── strategies/
│   │   ├── long.go                 # Стратегия Directional Long с проверкой паттерна "New Money"
│   │   ├── long_grid.go            # Стратегия Long Grid (восходящие каналы + запас до Resistance)
│   │   ├── neutral_grid.go         # Стратегия Neutral Grid (боковики, защиту от IL и Broadening Wedges)
│   │   ├── short.go                # Стратегия Directional Short с проверкой паттерна "Aggressive Shorting"
│   │   ├── short_grid.go           # Стратегия Short Grid (затухание пампа у Resistance + 15m/1h break)
│   │   └── strategy.go             # Интерфейс Strategy, фабричный конструктор и утилиты clamp/status
│   └── structure/
│       └── structure.go            # Детекция Pivot High/Low, определитель HighState/LowState и сортировка Levels
├── models/
│   └── market.go                   # Единая доменная модель данных (Candidate, Indicators, MarketData и др.)
├── output/
│   └── json.go                     # Атомарная запись JSON с дедупликацией по MD5-хэшу
├── go.mod                          # Модуль Go (1.26.5)
└── README.md                       # Техническая документация проекта
```
## Детальное описание ключевых компонентов

1. internal/bybit/ws_orderbook.go - Реализует транспортный слой для работы с L2-стаканами Bybit WebSocket V5 (wss://stream.bybit.com/v5/public/linear):
- OrderBookCache: Хранит локальные слепки стаканов (map[float64]float64 для bids и asks). Метод Update обрабатывает как первичные snapshot, так и дельты delta (удаляя уровни с size == 0).
- GetMetrics: Вычисляет объемы BidNotional / AskNotional, процентное смещение ImbalancePct и коэффициент BidAskRatio за $O(1)$.
- WSClient: Управляет неблокирующим чтением сообщений (readLoop), отправкой пингов каждые 20 секунд (keepAlive) и фоновой синхронизацией.

2. internal/strategies/ (Математика и Эвристика)
   Все стратегии реализуют единый интерфейс Strategy и принимают указатель на полностью собранный снимок кандидата (*models.Candidate):
- long.go: Требует бычью структуру на 1h/4h (HH/HL). Применяет жесткий заслон (reject) при обнаружении паттерна Short Covering (цена растет, а OI падает — искусственный вынос на стопах).
- short.go: Требует медвежью структуру на 1h/4h (LH/LL). Применяет reject при обнаружении паттерна Long Liquidation (цена падает и OI падает — каскадный сброс лонгов).
- neutral-grid.go: Ищет флетовые каналы. Применяет reject при наличии сильных трендов, аномального 24h роста, расширяющихся формациях (HH+LL — риск Impermanent Loss) и если ширина диапазона меньше $1.5 \times \text{ATR}_{4h}$.
- long_grid.go & short_grid.go: Оценивают пригодность ассета для сеточных ботов. Забраковывают монеты на ATH (без ближайшего сопротивления) или при подходе цены ближе чем на 20-30% к границе канала.

3. internal/analysis/service.go
   Оркестратор сбора данных и пайплайна:
- Выполняет быстрый REST-фильтр по минимальному обороту (min_turnover_24h) и максимальной цене.
- Формирует преселект-пул (preselect_candidates) и передает список тикеров в WSClient для подписки на orderbook.50.<symbol>.
- Ожидает 3 секунды для прогрева кэша стакана (наполнение первичными snapshot'ами).
- Запускает параллельный сбор свечей (15m, 1h, 4h), истории фандинга и OI с использованием пула горутин, ограниченного флагом concurrency.
- Снимает безблокировочные метрики стакана из OrderBookCache и вычисляет оценки во всех 5 стратегиях.

4. output/json.go
   Модуль атомарного сохранения результатов:
- riteJSONIfChanged: Вычисляет MD5-хэш от итоговой сериализованной структуры ScreeningResult. Если хэш совпадает с предыдущим тиком, операция записи пропускается, сохраняя ресурс диска.
- atomicWrite: Writes во временный файл .tmp в той же директории и выполняет атомарный os.Rename.

## Быстрый запуск
Требования
- Go 1.26.5 или совместимый;
- Сетевой доступ к публичным REST/WS эндпоинтам Bybit (api.bybit.com / stream.bybit.com).

Сборка
```bash
go build -o screener ./cmd/screener
```

### Запуск в режиме демона
Запуск со стандартным интервалом обновлений (3 минуты) и выбором стратегии через CLI:
```bash
./screener --strategy short --interval 3m
```

Прямой запуск конкретной стратегии с кастомным интервалом (1 минута):
```bash
./screener --strategy neutral-grid --interval 1m --config configs/config.json
```

### Доступные значения флага --strategy:
- short-grid
- short
- long-grid
- long
- neutral-grid

При работе приложение генерирует/обновляет итоговый файл в формате <strategy>-screening.json (например, neutral-grid-screening.json).



``` json
"bybit": {
"base_url": "[https://api.bybit.com](https://api.bybit.com)"
},
"filters": {
"max_price": 2.0,
"min_turnover_24h": 5000000,
"preselect_candidates": 60,
"top_candidates": 20,
"max_grid_spread_pct": 0.15
},
"analysis": {
"kline_limit_15m": 300,
"kline_limit_1h": 300,
"kline_limit_4h": 300,
"open_interest_limit": 50,
"funding_limit": 20,
"order_book_limit": 100
},
"concurrency": 8,
"http_timeout": "10s",
"run_timeout": "3m",
"max_retries": 3,
"retry_delay": "500ms",
"output": {
"file": "screening.json"
}
}
```
Валидация и тестированиеПеред развертыванием кода в продакшен рекомендуется запустить стандартный цикл проверок:

```azure
go fmt -w .
go vet ./...
go test ./...
```
