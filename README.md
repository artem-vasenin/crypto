# Universal Bybit Screener / Bot

## Что это

Проект состоит из двух исполняемых частей:

- `screener` — исследовательский/сигнальный скриннер USDT Linear Perpetual на Bybit;
- `bot` — исполнитель, который читает JSON скриннера и может выставлять реальные PostOnly-заявки на Bybit.

Скриннер не открывает позиции. Бот не рассчитывает рынок заново — он использует screening snapshot, но перед заявкой повторно проверяет актуальные bid/ask, instrument limits и ограничения риска.

## Стратегии скриннера

Доступны:

- `long`
- `short`
- `long-grid`
- `short-grid`
- `neutral-grid`

`long-grid`, `short-grid` и `neutral-grid` в этой версии являются **только screening strategies**. Торговый бот намеренно принимает только `long` и `short`, чтобы не превратить score grid-стратегии в ошибочную directional торговлю.

## Основные исправления

В исправленной версии устранены критические проблемы исходника:

- PostOnly теперь выставляется на maker-стороне spread: Buy по bid, Sell по ask;
- pending order не считается открытой позицией до фактического fill;
- добавлена синхронизация позиций и заявок после рестарта;
- добавлен order lifecycle через private WebSocket;
- зависшие pending orders автоматически отменяются;
- контролируется `MaxTotalMarginUSD`;
- используется `totalAvailableBalance` Unified Account вместо устаревшего `availableToWithdraw`;
- удалён вызов устаревшего `switch-isolated` из торгового пути;
- SL/TP проходят pre-trade проверку;
- SL/TP передаются уже при создании заявки;
- исправлен опасный fallback SL при отсутствии pivot;
- добавлен fee/cost safety check;
- анализируются закрытые свечи;
- исправлен off-by-one в VolumeRatio;
- новые символы Top-N автоматически добавляются в WebSocket universe;
- проверяется свежесть order book;
- neutral-grid существенно ужесточён;
- добавлены unit-тесты критической risk/strategy логики.

Подробности: `AUDIT.md`.

Подробное описание каждого файла и каждой функции: `STRUCTURE.md`.

## Требования

- Go 1.26.5
- Bybit V5 API
- API key/secret с разрешениями, необходимыми для торговли, если запускается `bot`

## Переменные окружения

Создайте `.env`:

```env
BYBIT_API_KEY=...
BYBIT_API_SECRET=...
```

Не добавляйте `.env` в Git.

## Сборка

Собрать оба бинарника:

```bash
make build
```

Отдельно:

```bash
go build -ldflags="-s -w" -o bot ./cmd/bot
go build -ldflags="-s -w" -o screener ./cmd/screener
```

Проверка:

```bash
go test ./...
go vet ./...
```

## Запуск скриннера

Long:

```bash
./screener -strategy long
```

Short:

```bash
./screener -strategy short
```

Long grid:

```bash
./screener -strategy long-grid
```

Short grid:

```bash
./screener -strategy short-grid
```

Neutral grid:

```bash
./screener -strategy neutral-grid
```

Изменить интервал:

```bash
./screener -strategy long -interval 5m
```

Использовать другой конфиг:

```bash
./screener -strategy long -config configs/config.json
```

Результат для `long`:

```text
long-screening.json
```

Для остальных стратегий имя строится как:

```text
<strategy>-screening.json
```

## Запуск бота

Перед реальной торговлей рекомендуется использовать Testnet.

Long:

```bash
./bot -strategy long -input long-screening.json
```

Short:

```bash
./bot -strategy short -input short-screening.json
```

Другой конфиг:

```bash
./bot \
  -strategy long \
  -config configs/config.json \
  -input long-screening.json
```

Бот намеренно завершится с ошибкой при попытке:

```bash
./bot -strategy neutral-grid
```

или:

```bash
./bot -strategy long-grid
```

Это сделано специально: grid screening и grid execution — разные вещи.

## Конфигурация риска

В `configs/config.json`:

```json
"execution": {
  "testnet": false,
  "max_leverage": 2,
  "margin_per_trade_usd": 3.0,
  "max_total_margin_usd": 12.0,
  "max_active_positions": 3,
  "min_score": 70.0,
  "trailing_pct": 1.0,
  "pending_order_timeout": "5m",
  "maker_fee_rate": 0.0002,
  "taker_fee_rate": 0.00055,
  "extra_cost_pct": 0.02,
  "max_stop_loss_pct": 5.0,
  "min_net_profit_pct": 0.5
}
```

`maker_fee_rate` и `taker_fee_rate` здесь являются консервативными настройками для расчёта защитного запаса, а не утверждением о конкретной комиссии вашего аккаунта.

## Что делает бот перед заявкой

1. Проверяет score.
2. Проверяет отсутствие уже открытой позиции по символу.
3. Проверяет cooldown.
4. Проверяет количество активных/pending позиций.
5. Проверяет общий лимит маржи.
6. Проверяет доступный баланс.
7. Получает свежий bid/ask.
8. Получает актуальные tick/qty/min-notional/max-qty ограничения.
9. Рассчитывает размер позиции.
10. Проверяет SL.
11. Проверяет TP после оценочных комиссий.
12. Устанавливает leverage.
13. Выставляет PostOnly limit order.
14. Передаёт SL/TP вместе с заявкой.
15. Далее состояние позиции синхронизируется по private WebSocket.

## Архитектура

```text
cmd/screener
      |
      v
internal/analysis
      |
      +--> internal/bybit
      +--> internal/indicators
      +--> internal/structure
      +--> internal/strategies
      |
      v
<strategy>-screening.json
      |
      v
cmd/bot
      |
      v
internal/execution
      |
      +--> public/private WebSocket
      +--> Bybit REST
      |
      v
Bybit
```

## Важное предупреждение

Это не доказательство того, что стратегия прибыльна.

Исправление программных ошибок не означает наличие положительного математического ожидания. Score, RSI, OI, funding, pivot levels и order-book imbalance сами по себе не доказывают, что сделка имеет edge.

Перед mainnet необходимо прогнать систему на Testnet и отдельно проверить:

- partial fills;
- отмену PostOnly;
- TP/SL после partial fill;
- WebSocket reconnect;
- restart/reconciliation;
- rate limits;
- реальные комиссии;
- минимальные размеры заявок;
- поведение при отсутствии liquidity;
- поведение при резком движении рынка.
