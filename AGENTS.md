# AGENTS.md — handoff Bybit Universal Deep Coin Analyzer v3.2

## 1. Цель

Проект — второй этап двухэтапной криптоаналитики. Массовый screener дешёво отбирает кандидатов; этот проект получает максимум 10 символов и строит глубокий evidence-first JSON по каждому. JSON предназначен прежде всего для последующей интерпретации LLM/человеком для GRID bot и обычных directional LONG/SHORT позиций.

Ключевой принцип: **программа не является торговым советчиком**. Она собирает raw evidence, вычисляет воспроизводимые derived metrics и структурированную иерархию evidence. Не добавлять жёсткий `OPEN LONG/SHORT` в analyzer.

## 2. Причина архитектуры

Предыдущая grid-логика переоценивала snapshot-признаки вроде LH/LL и верхней части диапазона. Сетка могла успешно собирать десятки/сотни циклов, но directional breakout создавал убыток больше grid-profit. Поэтому v3 сначала сохраняет доказательную базу и разделяет два разных вопроса: (a) подходит ли режим для mean-reverting GRID; (b) есть ли directional momentum/setup для обычной позиции. Сильный ADX/volume/volatility expansion может быть минусом для GRID и плюсом для directional.

## 3. Инварианты, которые нельзя случайно сломать

1. Raw evidence сохраняется всегда, независимо от меню.
2. Enter по первому меню = `full`; Enter по направлению = `both`.
3. Максимум 10 символов; `BTC` нормализуется в `BTCUSDT`.
4. Один символ = один самодостаточный JSON.
5. Ошибка одного символа/secondary endpoint не должна ломать весь сеанс.
6. Противоположные bullish/bearish/conflicting evidence сохраняются даже при запросе только LONG или SHORT.
7. `grid_analysis` и `directional_analysis` независимы.
8. Интегральные score запрещены без отдельно обученной и валидированной модели; analyzer не выдаёт OPEN-сигнал.
9. Все timestamps должны позволять проверить свежесть; `generated_at` — UTC.
10. Комментарии исходного кода и пользовательская документация — на русском языке.

## 4. Пользовательский поток

CLI: выбор стратегии `1 GRID / 2 Directional / 3 Full(default)` → направление `1 LONG / 2 SHORT / 3 Both(default)` → ввод 1–10 символов через запятую без пробелов → лёгкая проверка ticker → глубокий анализ валидных символов → отдельные JSON → `Enter/1` новый сеанс или `0` выход.

Выбор пользователя записывается в `request`, но не должен заставлять сборщик урезать raw evidence. Он нужен внешнему ИИ как контекст задачи.

## 5. Источники Bybit

Используются публичные V5 endpoints через `internal/bybit/client.go`: ticker, klines, mark/index klines, funding, open interest, long/short ratio, orderbook, recent public trades. API key не нужен. При расширении API всю сетевую логику держать в `internal/bybit`, а не размазывать по analysis.

## 6. Целевая глубина

OHLCV: 5m 864 (~72h), 15m 1344 (~14d), 1h 1440 (~60d), 4h 1080 (~180d), 1D 365 (~365d). OI и long/short: 5m ~24h. Funding: 90 записей. Orderbook: до 500 уровней. Recent trades: до 1000. BTC 1h: до 720 для 30d correlation. Фактические counts обязательно писать в `data_quality`.

## 7. JSON contract v3.2

`schema_version`, `analyzer_version`, `generated_at`, `exchange`, `category`, `symbol`, `request`, `purpose`.

`data_quality`: complete/warnings/counts/requested_depth.

`market`: текущая цена, 24h high/low/change/volume/turnover, spread, funding, OI.

`timeframes`: нейтральные RSI, ATR/ATR%, ADX, EMA20/50/200, Efficiency Ratio, realized vol, Bollinger width, volume ratio, window high/low/position.

`market_regime`: classification, trend strength, volatility state, breakout risk, directional bias, evidence/conflicts. Это описание, не торговый сигнал.

`range_analysis`: 48h range по 15m, high/low/mid/width/position, touches, midpoint crosses, false breaks, slope.

`grid_analysis`: regime/range/mean-reversion/breakout diagnostics + LONG/SHORT SideAssessment. При сильном directional expansion должны появляться risk factors/hard blocks, а не арифметический штраф.

`directional_analysis`: независимые LONG/SHORT SideAssessment, bullish/bearish/conflicting evidence, invalidation/target context. Momentum expansion здесь не должен автоматически штрафоваться как в GRID.

`derivatives`: raw funding/OI/long-short series + изменения.

`microstructure`: raw orderbook плюс агрегаты recent trades, taker delta, mark/index basis.

`btc_context`: изменения BTC, 30d 1h correlation, relative strength 24h.

`raw_evidence`: OHLCV всех TF, recent trades, mark/index history. Не удалять ради уменьшения JSON.

`ai_instructions`: контракт для внешнего ИИ. ИИ обязан проверять data_quality и может перепроверять derived metrics по raw evidence.

## 8. Алгоритмические блоки

`tf`: базовые индикаторы и геометрия окна.

`analyzeRange`: текущая реализация использует последние 192 свечи 15m (~48h). Touch tolerance = 8% ширины range. Mid-crosses и slope — признаки mean reversion/drift. Улучшения допустимы, но сохранять объяснимость и raw evidence.

`classifyRegime`: формирует нейтральный market regime и GRID diagnostics. Высокие ADX/Efficiency Ratio/volume expansion/range drift уменьшают grid suitability. Это эвристика, не ML и не probability.

`analyzeDirectional`: отдельная эвристика по EMA multi-TF, RSI, ADX, taker delta и OI. Её можно существенно улучшать: swing structure, breakout/retest, impulse/pullback, divergence, volatility-normalized invalidation, price-OI states. При этом не удалять противоположные evidence.

## 9. Приоритетные будущие улучшения

- Явные swing points с timestamp и HH/HL/LH/LL по 15m/1h/4h.
- ATR trend/volatility expansion как time series, а не только snapshot.
- Несколько candidate ranges разной длительности и их статистика устойчивости без сведения в произвольный score.
- Breakout/retest/failed breakout detection.
- Impulse vs pullback decomposition и relative volume каждого leg.
- Более глубокая OI-price state machine.
- Historical taker delta/CVD только если найден надёжный публичный источник; не притворяться, что последние 1000 trades = исторический CVD.
- Support/resistance clusters и volatility-normalized invalidation/targets.
- Tests с fixture MarketAPI без сети для Build/regime/directional.

## 10. Правила изменения

Перед коммитом: `gofmt`, `go test ./...`, `go vet ./...`, `go build ./cmd/analyzer`. При изменении JSON обновлять schema version, README и этот файл. При добавлении функции добавить русский комментарий и упомянуть её в README. Не скрывать ошибки API: secondary ошибки → warnings, критическая OHLCV ошибка → ошибка конкретного символа.

## 11. Структура проекта

`cmd/analyzer/main.go` CLI; `internal/bybit/client.go` сеть/DTO; `internal/indicators/indicators.go` чистая математика; `internal/analysis/types.go` JSON contract; `internal/analysis/build.go` orchestration/derived analysis; `internal/output/json.go` сериализация; tests; README; AGENTS; go.mod.

## 12. Как внешний ИИ должен использовать JSON

Сначала data quality/freshness → затем raw OHLCV/regime → для GRID range stability/mean reversion/breakout risk → для directional trend/impulse/pullback/breakout/retest → derivatives/orderflow confirmation → противоположные evidence → только затем решение. Не пытаться свести evidence к псевдовероятности. Финальные entry/range/SL/TP должны быть обоснованы уровнями и raw evidence.

## Архитектурное решение v3.2: НИКАКИХ интегральных score

Не возвращать арифметические score без отдельно обученной и валидированной модели весов. Разнородные технические, структурные, деривативные и микроструктурные признаки не являются взаимозаменяемыми баллами. В v3.2 используется evidence hierarchy: `hard_blocks` > `primary_evidence` > `secondary_evidence`, а `risk_factors/conflicts` сохраняются отдельно. Raw evidence всегда является доступным источником для перепроверки.

Новые ключевые блоки: `structures` (реальные swing coordinates), `regime_history`, расширенный `range_analysis.stationarity`, `derivatives` с multi-window changes и `microstructure` с реальным coverage последних 1000 trades. Не интерпретировать snapshot order book или короткое окно trades как часовую историю.

При будущих изменениях нельзя снова вводить `100/100`, `score`, суммирование RSI+ADX+EMA и подобные псевдоточности. Если появится статистически обученная модель, её прогноз должен быть отдельным экспериментальным блоком с версией модели, датасетом/периодом валидации и calibration metrics, не заменяя evidence.

## Изменения и инварианты v3.2

После live-теста BTC/ETH добавлены три обязательных правила.

1. **Нельзя подменять временное окно количеством trades.** `recent-trade limit=1000` — это snapshot последних 1000 сделок, а не гарантированные 1/5/15 минут. `FlowWindow.Available` разрешается выставлять только при фактическом покрытии полного окна. Недостаточное окно должно возвращать `status=insufficient_data`, нулевые агрегаты окна и фактическое покрытие. Общий snapshot при этом сохраняется в raw evidence.
2. **Нельзя использовать неполный taker-flow в directional evidence.** В текущей реализации directional слой использует только полноценный 1m `FlowWindow`; если его нет, taker delta считается неизвестным, а не нейтрально подтверждённым.
3. **Outside-bar не формирует market structure.** Если одна свеча одновременно удовлетворяет pivot-high и pivot-low, обе координаты сохраняются с `ambiguous_outside_bar=true`, но исключаются из HH/HL/LH/LL. В `StructureAnalysis.conflicts` записывается факт исключения.
4. **`RANGING` без уточнения стационарности больше не использовать.** Базовые labels: `RANGING_STATIONARY`, `RANGING_DRIFT_UP`, `RANGING_DRIFT_DOWN`; более приоритетные `TRENDING_EXPANSION`, `BREAKOUT_RISK`, `TRENDING`, `COMPRESSION` сохраняют приоритет классификатора.

Не пытаться «исправить» отсутствие 5m/15m trade history повторными вызовами public recent-trade, если API не предоставляет историческую пагинацию для linear: это создаст дубликаты текущего snapshot, а не историю. Для настоящего непрерывного order-flow в будущем нужен отдельный live collector/WebSocket с локальным накоплением данных.
