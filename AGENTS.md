# AGENTS.md — Candidate Screener

## 1. Назначение

Этот файл — полный handoff для ИИ/разработчика. Проект является **самостоятельным Candidate Screener для Bybit USDT Perpetuals**. Он не открывает сделки, не создаёт/управляет ботами, не ведёт позиции и не заменяет Deep Coin Analyzer. Его задача — дешёво просканировать широкий рынок, построить единое объективное описание каждой монеты и отобрать только непротиворечивые кандидаты для последующего анализа ИИ и отдельным Deep Analyzer.

## 2. Главные архитектурные правила

1. Проект создаётся с нуля; старый trading-bot код не является архитектурной базой.
2. Один общий pipeline данных/features обслуживает все направления.
3. Сначала определяется состояние рынка: `Direction + Strength + Dynamics + MTF alignment`. Только потом стратегия интерпретирует это состояние.
4. Нет aggregate/additive score. Нельзя складывать RSI, ADX, proximity, structure и другие признаки в единый балл допуска.
5. Иерархия evidence: **hard gates > primary evidence > confirmations > risk flags**. Hard block нельзя компенсировать вторичными плюсами.
6. Противоречивое/неопределённое направление не попадает ни в LONG, ни в SHORT output.
7. Нет TOP-N и квот. 0 кандидатов — нормальный результат; 20 кандидатов тоже нормальный результат.
8. Grid и Futures используют одни market features, но разные правила пригодности.
9. Для Grid особенно важно не торговать против движущегося equilibrium. `oscillating != stationary`, `overextended != reversing`, `at resistance != rejected`.
10. Thresholds конфигурируемы и должны калиброваться на расширяющейся выборке. Не подгонять их под несколько исторических провалов.

## 3. Режимы

### FUTURES (по умолчанию)
Выходы: `screening-long.json`, `screening-short.json`. Дорогие ликвидные инструменты (например BTC) допустимы. Результат означает **кандидат на глубокий анализ**, а не команду открыть позицию.

### GRID
Выходы: `screening-grid-long.json`, `screening-grid-short.json`. Neutral Grid намеренно отсутствует. Дополнительно применяются grid-совместимость режима и capital suitability для небольшого капитала.

### Запуск
`go run ./cmd/screener` — автоматический FUTURES.
`go run ./cmd/screener --mode grid` — автоматический GRID.
`go run ./cmd/screener --interactive` — ручное меню.

## 4. Data flow

`Bybit public REST -> Universe -> candles/ticker -> Feature Engine -> MarketSnapshot -> Futures/Grid Analyzer -> Candidate -> JSON`.

Data layer не принимает торговых решений. Feature layer вычисляет факты. Screening layer интерпретирует факты согласно профилю. Output layer только сериализует результат.

## 5. Direction Engine v1

Ключевые TF: 15m, 1h, 4h. Direction не определяется одним индикатором. Используются независимые evidence: swing structure HH/HL/LH/LL, drift центра, DMI/ADX, efficiency, momentum и MTF согласованность.

Состояния direction: `UP`, `DOWN`, `SIDEWAYS`, `TRANSITION`, `UNKNOWN`.
Strength: `WEAK`, `MODERATE`, `STRONG`.
Dynamics: `ACCELERATING`, `STABLE`, `DECELERATING`.
MTF: `ALIGNED`, `TRANSITION`, `CONFLICT`, `MIXED`.

`TRANSITION` и `CONFLICT` не должны автоматически превращаться в directional candidate. Особо учитывать HTF lag: старый 4h trend может конфликтовать с устойчивым recovery/reversal на 15m/1h.

## 6. Grid-методология

Grid suitability не равна directional suitability. Сильный accelerating trend может быть хорошим FUTURES candidate и одновременно `NO_GRID`.

Grid v1 проверяет направление, отсутствие MTF conflict, отсутствие сильного ускоряющегося режима, ограничивает чрезмерный normalized center drift/efficiency и проверяет capital suitability. Это начальная консервативная реализация, а не окончательная модель entry/range/SL.

Будущая эволюция Grid должна идти по цепочке `GRID_ELIGIBILITY -> SETUP -> WAIT/TRIGGER -> READY_FOR_DEEP_ANALYSIS`. Boundary proximity сама по себе не trigger.

## 7. Исторические ошибки, которые нельзя вернуть

Регрессионные кейсы из предыдущей системы: AVAXUSDT, WIFUSDT, 1000PEPEUSDT, VVVUSDT SHORT-grid. Общий pathology: положительный impulse/recovery ошибочно принимался за возможность SHORT; локальные fills существовали, но equilibrium двигался вверх. VVV особенно показывает `local oscillation quality high, equilibrium stability bad`. PEPE показывает `pullback != reversal`. WIF/AVAX показывают HTF lag.

Когда появятся T0 fixtures этих сделок, добавить replay tests без future leakage. Не использовать текущий order book/recent trades как исторические данные T0.

## 8. Evidence quality

Missing/insufficient data не трактуется как 0 или нейтральное подтверждение. При недостаточной истории Direction=`UNKNOWN`, DataQuality=`INSUFFICIENT`, кандидат блокируется. Любые будущие taker-flow окна должны иметь фактическую coverage-проверку.

## 9. Структура проекта

- `cmd/screener/main.go` — CLI, меню, orchestration.
- `internal/config` — конфигурация и thresholds.
- `internal/domain` — общие DTO/enums; не содержит бизнес-логики.
- `internal/bybit` — public REST V5 client.
- `internal/universe` — дешёвые предварительные фильтры.
- `internal/indicator` — атомарная математика индикаторов.
- `internal/structure` — swing/HH-HL/LH-LL classification.
- `internal/features` — feature extraction и market classification.
- `internal/marketdata` — сбор MarketSnapshot.
- `internal/screening` — hierarchical strategy gates FUTURES/GRID.
- `internal/output` — JSON serialization.
- `testdata` — будущие deterministic/replay fixtures.

## 10. Правила изменения проекта

- Комментарии к коду писать на русском и объяснять прежде всего «почему».
- Новые признаки сначала добавлять как атомарные features; только затем определять их роль в evidence hierarchy.
- Нельзя добавлять `total_score`, `rank_score`, weighted sum или скрытый аналог aggregate score для допуска.
- Нельзя заставлять систему возвращать фиксированное число кандидатов.
- Hard gate должен иметь отдельный тест, доказывающий, что confirmations не могут его компенсировать.
- Grid-specific фильтры не должны загрязнять Futures analyzer.
- Business logic не переносить в CLI/Bybit/output.
- Live API tests отделять от deterministic tests.
- При изменении JSON schema обновлять `schema_version`, README и тесты.

## 11. Что намеренно НЕ решает v1

Нет БД, historical snapshots, Virtual Trader, исполнения сделок, автоматического выбора leverage, grid range/SL/TP, полноценного entry trigger engine, historical taker collector и сложной микроструктуры. Эти части нельзя незаметно добавлять в ядро без отдельного проектного решения.

## 12. Definition of Done для следующих итераций

При развитии проекта сохранять: корректное направление важнее точности точки входа; conflicting data не выходит в shortlist; Grid способен блокировать drifting/expanding equilibrium; Futures не блокируется только из-за высокой абсолютной цены; output объясним; tests deterministic; Deep Analyzer получает shortlist, а не OPEN-команду.
