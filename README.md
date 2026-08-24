# Bybit Monitor

Read-only монитор открытых позиций Bybit.

Приложение получает первоначальное состояние через REST API,
а дальнейшие изменения получает через private WebSocket.

Проект поддерживает:

- Bybit Mainnet
- Bybit Testnet
- HMAC authentication
- REST API
- private WebSocket
- WebSocket authentication
- WebSocket subscription
- heartbeat
- reconnect
- position opened events
- position updated events
- position closed events
- изменение Take Profit
- изменение Stop Loss
- изменение размера позиции
- изменение Mark Price
- изменение Unrealised PnL
- изменение Leverage

---

## Архитектура

Приложение разделено на три основные части.

### config

Отвечает за конфигурацию приложения.

Не знает ничего о WebSocket и Monitor.

### bybit

Отвечает за взаимодействие с Bybit API.

Внутри находятся:

- REST client
- WebSocket client
- API models
- JSON parsing
- HMAC authentication

### monitor

Отвечает за состояние позиций.

Monitor не знает, откуда пришли данные.

Он получает:

    bybit.Position

и определяет:

    opened
    updated
    closed

---

## Mainnet

Конфигурация:

    configs/config.json

REST:

    https://api.bybit.com

WebSocket:

    wss://stream.bybit.com/v5/private

---

## Testnet

Конфигурация:

    configs/config.testnet.json

REST:

    https://api-testnet.bybit.com

WebSocket:

    wss://stream-testnet.bybit.com/v5/private

Запуск:

    CONFIG_PATH=configs/config.testnet.json go run ./cmd/monitor

---

## Переменные окружения

Создать:

    .env

Пример:

    BYBIT_API_KEY=xxxxxxxx
    BYBIT_API_SECRET=xxxxxxxx

Реальные ключи нельзя помещать в Git.

---

## Запуск

Обычный запуск:

    go run ./cmd/monitor

Testnet:

    CONFIG_PATH=configs/config.testnet.json go run ./cmd/monitor

Race detector:

    go run -race ./cmd/monitor

---

## Тесты

Обычные тесты:

    go test ./...

Подробный вывод:

    go test ./... -v

Race detector:

    go test -race ./... -v

---

## Синхронизация состояния

При запуске используется следующая схема:

    REST
      |
      v
    текущие позиции
      |
      v
    Monitor
      |
      v
    WebSocket
      |
      v
    изменения позиций

Это важно.

WebSocket не должен рассматриваться как источник
полного первоначального состояния.

REST сначала сообщает:

    какие позиции существуют сейчас.

После этого WebSocket сообщает:

    что изменилось.

---

## События

### opened

Позиция появилась.

Например:

    XRPPERP Buy size=6.6

### updated

Позиция уже существовала,
но одно или несколько её значений изменились.

Например:

    TakeProfit: 0 -> 1.6986

или:

    Size: 6.6 -> 3.3

### closed

Bybit прислал:

    size = 0

После этого Monitor удаляет позицию
из текущего состояния.

---

## PositionIdx

Позиция идентифицируется комбинацией:

    Symbol
    PositionIdx

Это важно для hedge mode.

Например:

    BTCUSDT + PositionIdx 1
    BTCUSDT + PositionIdx 2

могут представлять разные позиции.

---

## Почему цены хранятся как string

Bybit возвращает цены и размеры
как строки.

Например:

    "0.03274"

Мы сохраняем их как string,
чтобы не получать неожиданные ошибки
округления float64.

Преобразование в decimal/float можно
добавить позже там, где оно действительно нужно.

---

## Read-only

Текущая версия проекта НЕ выполняет торговые операции.

Она:

- читает позиции
- получает WebSocket события
- хранит состояние
- сообщает об изменениях

Она не:

- создаёт ордера
- отменяет ордера
- меняет TP
- меняет SL
- закрывает позиции
- изменяет leverage

Это сделано намеренно.

Сначала строим надёжный монитор.

После этого торговую часть можно добавить отдельным
слоем и тестировать исключительно на Testnet.

---

## Следующий этап

После стабилизации Monitor можно добавить:

1. Order stream
2. Execution stream
3. Wallet stream
4. историю событий
5. хранение состояния
6. Telegram notifications
7. REST API
8. web interface
9. торговый модуль

Торговый модуль должен быть отделён от Monitor.

Monitor должен оставаться read-only источником состояния.