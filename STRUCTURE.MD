# STRUCTURE.md — архитектура Universal Bybit Screener / Bot

## 1. Главный принцип

Проект разделён на две подсистемы:

```text
Bybit market data
       │
       ▼
    SCREENER
       │
       ▼
 strategy-screening.json
       │
       ▼
      BOT
       │
       ▼
 Bybit execution
```

Скринер отвечает за исследование и отбор кандидатов. Bot отвечает за повторную safety-проверку и исполнение.

**Directional strategy больше не использует один общий score.**

---

## 2. Pipeline анализа

```text
REST + public WS
      │
      ├── price / 24h / liquidity
      ├── 5m / 15m / 30m / 1h / 4h candles
      ├── RSI / ATR / volume
      ├── HH / HL / LH / LL
      ├── support / resistance
      ├── funding
      ├── open interest
      └── order book
      │
      ▼
Candidate
      │
      ▼
Directional block analysis
      │
      ├── Trend Quality
      ├── Asset Quality
      ├── Entry Quality
      ├── Market Quality
      └── Derivatives Quality
      │
      ▼
Decision.Eligible
      │
      ▼
JSON
```

---

# 3. Блоки directional analysis

## 3.1 `TrendQuality`

Самый высокий приоритет.

Источники:

- 1h structure;
- 4h structure;
- 30m structure.

Направление:

```text
HH + HL = bullish
LH + LL = bearish
otherwise = neutral/mixed
```

Это не просто штраф к score. Для directional execution конфликтующий старший тренд является gate.

Примеры:

```text
long + 1h LH/LL -> reject
long + 4h LH/LL -> reject
short + 1h HH/HL -> reject
short + 4h HH/HL -> reject
```

Это сделано именно для устранения ситуации, когда сильный 5m/15m сигнал перекрывал плохой 1h/4h контекст.

---

## 3.2 `AssetQuality`

Описывает, насколько сам актив поддерживает выбранное направление на более широком горизонте.

Использует:

- 24h change;
- 3d change;
- 7d change;
- RSI 1h;
- RSI 4h.

Это отвечает на вопрос:

> «Мы вообще хотим торговать этот актив в выбранную сторону?»

---

## 3.3 `EntryQuality`

Описывает момент входа.

Использует:

- 5m structure;
- 15m structure;
- RSI 5m;
- RSI 15m;
- position внутри 1h range.

Это отвечает на другой вопрос:

> «Даже если актив подходит для long/short, хорошо ли входить прямо сейчас?»

`EntryQuality` не имеет права самостоятельно отменять плохой macro context.

---

## 3.4 `MarketQuality`

Использует:

- spread;
- 24h turnover;
- 1h volume ratio;
- 1h volume trend.

Цель — не искать направление, а проверять качество торговой среды.

---

## 3.5 `DerivativesQuality`

Использует:

- current funding;
- OI change;
- order-book imbalance.

Этот блок имеет более низкий приоритет, чем trend/asset/entry. Он не должен превращать плохую монету в хорошую только потому, что стакан временно перекошен.

---

# 4. Directional decision

`StrategyResult` содержит:

```text
Scores
Decision
Status
Reason
```

`Scores`:

```text
AssetQuality
TrendQuality
EntryQuality
MarketQuality
DerivativesQuality
```

`Decision`:

```text
Eligible
Priority
BlockingReasons
```

Текущие минимумы:

```text
TrendQuality       >= 60
AssetQuality       >= 55
EntryQuality       >= 60
MarketQuality      >= 60
DerivativesQuality >= 40
```

Приоритет:

```text
1. TrendQuality
2. AssetQuality
3. EntryQuality
4. MarketQuality
5. DerivativesQuality
```

Нет операции вида:

```text
score = trend + RSI + volume + OI + orderbook
```

Это принципиальное архитектурное решение.

---

# 5. Screening ranking

После анализа результаты сортируются так:

1. `Eligible=true` выше `Eligible=false`;
2. внутри группы — `EntryQuality`;
3. затем — `AssetQuality`.

Это позволяет top candidates отражать качество реальных торговых возможностей, а не просто сумму разнородных сигналов.

---

# 6. Grid strategies

`long-grid`, `short-grid`, `neutral-grid` пока сохраняют собственную suitability-модель.

Они являются screening-only.

Bot никогда не должен использовать их для directional execution.

`neutral-grid` имеет отдельные gates:

- отсутствие явного 1h/4h directional trend;
- допустимый ATR;
- достаточная ширина range относительно ATR;
- цена не у края диапазона.

---

# 7. Execution pipeline

```text
screening JSON
      │
      ▼
Decision.Eligible?
      │
      ▼
screening age check
      │
      ▼
position / cooldown / margin limits
      │
      ▼
fresh bid/ask
      │
      ▼
instrument limits
      │
      ▼
leverage + quantity
      │
      ▼
SL / TP calculation
      │
      ▼
risk validation
      │
      ▼
PostOnly order with TP/SL
```

---

# 8. Screening freshness

Bot принимает только screening snapshot не старше `max_screening_age`.

Default:

```text
3m
```

Причина: повторный live bid/ask не обновляет старые RSI, structure, levels и derivatives.

---

# 9. Risk model

## Stop Loss

`CalculateDynamicStopLoss`:

- использует pivot соответствующего направления;
- использует минимум `1.5 * ATR1h`;
- проверяет максимальный допустимый SL;
- проверяет, что SL находится с правильной стороны entry.

## Take Profit

`CalculateDynamicTakeProfit`:

- минимум RR = 2;
- long: TP выше entry;
- short: TP ниже entry.

`calculateRiskLevels` дополнительно вычитает оценочные торговые расходы.

В short была исправлена ошибка расчёта процентной дистанции: `TP-entry` должен использоваться по модулю.

---

# 10. TP/SL lifecycle

Основная PostOnly заявка содержит TP/SL:

```text
Full
MarkPrice
TP Market
SL Market
```

После fill второй `trading-stop` с теми же уровнями не отправляется.

Причина: во время реального запуска Bybit возвращал `34040 not modified`, потому что risk levels уже были заданы на основной заявке.

Локальное состояние после успешного создания заявки:

```text
RiskAttached = true
```

Execution WebSocket используется для:

- подтверждения фактического fill;
- snapshot logging;
- проверки стороны/order ID.

---

# 11. Managed state

`PositionState.Managed` разделяет:

```text
managed=true
```

позиции/заявки, созданные текущим bot instance, и

```text
managed=false
```

чужие или найденные при startup reconciliation.

Unmanaged positions:

- не управляются trailing;
- не получают snapshot как вход текущего бота;
- не используются как собственные execution states;
- при этом учитываются safety logic, чтобы бот не превышал лимиты аккаунта.

---

# 12. Long + Short одновременно

Оба экземпляра получают один private execution stream.

Поэтому execution обрабатывается только если:

```text
position exists
AND
Managed=true
AND
OrderID matches
AND
execution side matches bot target side
```

При конфликте стороны символ блокируется для текущего engine и пишется `[CRITICAL]`.

---

# 13. Snapshots

Snapshot создаётся только после реального `execution`.

Сохраняются:

- symbol;
- side;
- actual execution price;
- actual execution quantity;
- leverage;
- order ID;
- execution ID;
- execution fee;
- execution time;
- full Candidate;
- BTC 15m context.

Это основа дальнейшего research dataset.

---

# 14. Пакеты

## `cmd/screener`

CLI entrypoint скриннера.

## `cmd/bot`

CLI entrypoint торгового движка.

## `config`

Загрузка JSON runtime-конфигурации.

## `models`

Domain models рынка, анализа и execution state.

## `internal/bybit`

REST и WebSocket интеграция Bybit.

## `internal/indicators`

RSI, ATR, volume metrics.

## `internal/structure`

Pivot/HH/HL/LH/LL и support/resistance.

## `internal/strategies`

Стратегии и directional block scoring.

Основные файлы:

- `strategy.go` — общие scoring helpers и decision gates;
- `long.go` — long block analysis;
- `short.go` — short block analysis;
- `long_grid.go` — long-grid suitability;
- `short_grid.go` — short-grid suitability;
- `neutral_grid.go` — neutral-grid suitability.

## `internal/analysis`

Собирает Candidate и применяет стратегии.

## `internal/execution`

Ордеры, risk, positions, WebSocket state и snapshots.

## `output`

Атомарная запись screening JSON.

---

# 15. Тестирование

Основные команды:

```bash
go test ./...
go vet ./...
go build ./cmd/screener
go build ./cmd/bot
```

Особенно важные тесты:

- регистрация всех стратегий;
- rejection long при конфликте 1h/4h;
- возможность short при согласованных блоках;
- neutral-grid rejection directional trend;
- SL direction;
- TP direction;
- short TP positive distance;
- position sizing exchange minimum;
- PostOnly + TP/SL parameters;
- target side long/short.

---

# 16. Research roadmap

Система специально не пытается сейчас «найти идеальные веса».

Следующий цикл:

```text
real entry snapshot
       ↓
trade outcome
       ↓
block-by-block analysis
       ↓
find repeatable patterns
       ↓
adjust gates/thresholds
       ↓
collect new sample
```

Нужно накапливать выборку, а не подгонять стратегию под 1–2 сделки.
