# Universal Bybit Screener / Bot

Проект разделяет **исследование рынка** и **исполнение сделок**. Скринер строит структурированный snapshot кандидатов, а бот принимает решение только по уже рассчитанным блокам анализа и повторно проверяет торговые ограничения перед заявкой.

> **Важно:** это исследовательский/экспериментальный торговый код. Перед mainnet обязательно тестировать на Testnet и отдельно проверять фактические ответы Bybit для вашего аккаунта.

## Главное архитектурное изменение

В проекте **больше нет одного общего score**, который складывает разнотипные сигналы в одно число.

Для directional-стратегий (`long` / `short`) анализ разделён на независимые блоки:

1. `trend_quality` — приоритетный блок. 1h/4h/30m HH/HL/LH/LL.
2. `asset_quality` — положение актива в более широком движении: 24h/3d/7d и RSI 1h/4h.
3. `entry_quality` — качество входа сейчас: 5m/15m, RSI, положение внутри диапазона.
4. `market_quality` — ликвидность, spread и объём.
5. `derivatives_quality` — funding, OI и дисбаланс стакана.

Решение строится **последовательно**, а не через сумму:

```text
TREND / MACRO
    ↓
ASSET QUALITY
    ↓
ENTRY QUALITY
    ↓
MARKET QUALITY
    ↓
DERIVATIVES
    ↓
RISK / EXCHANGE CHECKS
    ↓
OPEN ORDER
```

Поэтому сильный 5m/15m сигнал больше не может автоматически компенсировать плохую 1h/4h структуру.

### Минимальные directional gates

- `trend_quality >= 60`
- `asset_quality >= 55`
- `entry_quality >= 60`
- `market_quality >= 60`
- `derivatives_quality >= 40`

Дополнительно:

- явный bearish `1h` или `4h` блокирует `long`;
- явный bullish `1h` или `4h` блокирует `short`;
- экстремальный spread/ATR блокирует вход;
- неподходящий RSI 1h блокирует вход.

Это **исследовательские пороги**, а не доказанная прибыльная стратегия. Их задача сейчас — не оптимизировать прибыль, а устранить архитектурную ошибку смешивания конфликтующих сигналов.

## Формат результата screener

Каждая directional strategy теперь содержит примерно такую структуру:

```json
{
  "scores": {
    "asset_quality": {
      "score": 55,
      "status": "weak"
    },
    "trend_quality": {
      "score": 33,
      "status": "poor"
    },
    "entry_quality": {
      "score": 80,
      "status": "strong"
    },
    "market_quality": {
      "score": 75,
      "status": "strong"
    },
    "derivatives_quality": {
      "score": 70,
      "status": "acceptable"
    }
  },
  "decision": {
    "eligible": false,
    "priority": [
      "trend_quality",
      "asset_quality",
      "entry_quality",
      "market_quality",
      "derivatives_quality"
    ],
    "blocking_reasons": [
      "long blocked: trend quality 33 < 60",
      "long blocked: 4h structure is bearish (LH+LL)"
    ]
  }
}
```

Таким образом AI и последующий research видят **почему** монета прошла или не прошла, а не только потерявшее смысл число `80`.

## Стратегии

- `long` — directional long screening + execution.
- `short` — directional short screening + execution.
- `long-grid` — screening only.
- `short-grid` — screening only.
- `neutral-grid` — screening only.

Grid стратегии не используются торговым ботом для открытия directional позиции.

## Сборка

Требуемая версия проекта: **Go 1.26.5+**.

```bash
go test ./...
go vet ./...
go build ./cmd/screener
go build ./cmd/bot
```

В текущем окружении разработки может быть только более старая версия Go. В таком случае её нельзя считать полноценной проверкой Go 1.26.5; синтаксис и тесты можно дополнительно проверять на локальном Go 1.26.5.

## Запуск screener

```bash
./screener -strategy long
./screener -strategy short
./screener -strategy long-grid
./screener -strategy short-grid
./screener -strategy neutral-grid
```

Результат:

```text
long-screening.json
short-screening.json
...
```

## Запуск bot

```bash
./bot -strategy long -input long-screening.json
./bot -strategy short -input short-screening.json
```

Bot принимает только `long` и `short`.

Перед открытием позиции бот:

1. требует `decision.eligible=true`;
2. проверяет возраст screening snapshot;
3. проверяет отсутствие позиции/ожидающей заявки по символу;
4. учитывает общий лимит активных позиций и маржи;
5. получает свежие bid/ask;
6. получает актуальные instrument limits;
7. рассчитывает leverage и размер позиции;
8. рассчитывает SL/TP;
9. проверяет SL и TP относительно ATR и лимитов риска;
10. создаёт PostOnly order.

### Защита от устаревшего анализа

`max_screening_age` по умолчанию равен `3m`. Старый screening JSON не используется для открытия новой позиции.

Это важно: свежий bid/ask сам по себе не делает старые RSI/структуру/уровни свежими.

## Risk management

В `configs/config.json`:

```json
"execution": {
  "max_leverage": 2,
  "margin_per_trade_usd": 3.0,
  "max_total_margin_usd": 12.0,
  "max_active_positions": 3,
  "trailing_pct": 1.0,
  "pending_order_timeout": "5m",
  "maker_fee_rate": 0.0002,
  "taker_fee_rate": 0.00055,
  "extra_cost_pct": 0.02,
  "max_stop_loss_pct": 5.0,
  "min_net_profit_pct": 0.5,
  "max_screening_age": "3m"
}
```

### SL/TP

SL строится от соответствующего уровня и минимум от ATR. TP строится от фактической цены заявки с минимальным RR 2:1.

Для short обязательно проверяется направление:

```text
Sell:
entry > TP
SL > entry
```

Исправлена ошибка, из-за которой отрицательное расстояние `TP-entry` для short могло попасть в расчёт как отрицательный процент.

### TP/SL Bybit

TP/SL передаются **сразу при создании основной PostOnly заявки** в `Full` режиме с `MarkPrice` и market execution.

После фактического fill бот **не вызывает второй раз `trading-stop` с теми же уровнями**. Это устраняет наблюдавшийся `34040 not modified` и оставляет локальный `RiskAttached=true`, чтобы trailing stop мог работать.

Фактический execution всё равно отдельно фиксируется через private WebSocket.

## Снэпшоты реальных входов

Снэпшот создаётся только после фактического `execution` события Bybit.

В него входят:

- фактическая цена исполнения;
- фактическое количество;
- leverage;
- order ID;
- execution ID;
- execution fee;
- время исполнения;
- полный Candidate;
- BTC 15m context.

Это позволяет позже сопоставлять **условия в момент входа** с фактическим результатом сделки.

## Одновременный long + short

Оба бота получают общий private execution stream Bybit. Поэтому локальное состояние содержит `Managed` и проверяет `OrderID` и сторону execution.

Чужие позиции/заявки:

- видны боту;
- учитываются в safety limits;
- не управляются trailing/risk логикой другого экземпляра.

Это особенно важно при одновременном запуске long и short на одном аккаунте.

## Исследовательский принцип

Сейчас не следует оптимизировать веса под несколько первых сделок.

Следующий этап исследования:

1. собирать snapshots реальных входов;
2. фиксировать результат сделки;
3. сравнивать каждый блок отдельно с outcome;
4. искать повторяющиеся признаки успешных/неуспешных входов;
5. только после накопления выборки менять пороги.

Главный вопрос теперь не «какой общий score был у монеты?», а:

> **была ли монета качественным активом, совпадал ли старший тренд с направлением, был ли хороший вход и позволял ли риск/рынок открыть сделку?**
