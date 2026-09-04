```bash
go build -o screener ./cmd/screener
```

```bash
go build -o bot ./cmd/bot 
```

```bash
./screener --strategy long --interval 1m --config configs/config.json
```

```bash
./screener --strategy short --interval 1m --config configs/config.json
```

```bash
./bot --strategy long --input long-screening.json --config configs/config.json
```

```bash
./bot --strategy short --input short-screening.json --config configs/config.json
```

# Чек-лист и шпаргалка по управлению инфраструктурой через SSH

## 1. Сборка (Компиляция) из исходников

Выполняется из рабочей директории `/opt/trading-bot` при внесении изменений в код.

```bash
cd /opt/trading-bot

# Подтянуть изменения из репозитория
git pull

# Сборка бинарника скринера
go build -ldflags="-s -w" -o screener ./cmd/screener

# Сборка бинарника исполняющего движка
go build -ldflags="-s -w" -o bot ./cmd/bot
```

## 2. Управление Торговыми Ботами (bot-long и bot-short)
Перепрочесть настройки автоматики
```bash
systemctl daemon-reload
```

Проверка статуса

```bash
# Статус обоих ботов
systemctl status bot-long bot-short

# Статус только Long-бота
systemctl status bot-long

# Статус только Short-бота
systemctl status bot-short
```

Запуск
```bash
# Запуск обоих ботов
systemctl start bot-long bot-short

# Запуск только Long / Short
systemctl start bot-long
systemctl start bot-short
```

Остановка
```bash
# Остановка обоих ботов
systemctl stop bot-long bot-short

# Остановка только Long / Short
systemctl stop bot-long
systemctl stop bot-short
```

Перезапуск
```bash
# Перезапуск обоих ботов
systemctl restart bot-long bot-short

# Перезапуск конкретного бота
systemctl restart bot-long
systemctl restart bot-short
```

## 3. Управление Скринерами (screener-long и screener-short)
Проверка статуса
```bash
systemctl status screener-long screener-short
```

Запуск
```bash
systemctl start screener-long screener-short
```

Остановка
```bash
systemctl stop screener-long screener-short
```

Перезапуск
```bash
systemctl restart screener-long screener-short
```

## 4. Массовое управление всей экосистемой (Скринеры + Боты)
Полный перезапуск (после компиляции)
```bash
systemctl restart screener-long screener-short bot-long bot-short
```

Полная остановка
```bash
systemctl stop screener-long screener-short bot-long bot-short
```

Полный запуск
```bash
systemctl start screener-long screener-short bot-long bot-short
```

Статус всей системы в один экран
```bash
systemctl status screener-long screener-short bot-long bot-short
```

## 5. Просмотр и Мониторинг Логов
В реальном времени (live-tail через log-файлы)
```bash
# Логи Long-бота
tail -f /opt/trading-bot/logs/long-bot.log

# Логи Short-бота
tail -f /opt/trading-bot/logs/short-bot.log

# Логи Long-скринера
tail -f /opt/trading-bot/logs/screener-long.log

# Логи Short-скринера
tail -f /opt/trading-bot/logs/screener-short.log

# Логи ошибок (Error logs)
tail -f /opt/trading-bot/logs/short-bot-error.log
tail -f /opt/trading-bot/logs/long-bot-error.log
```

Посмотреть последние 100 строк лога
```bash
tail -n 100 /opt/trading-bot/logs/short-bot.log
tail -n 100 /opt/trading-bot/logs/long-bot.log
```


## 6. Типовой Workflow применения доработок (One-liner)
Если внесены правки в код, команда полных пересборки и перезапуска в одну строку:
```bash
cd /opt/trading-bot && git pull && go build -ldflags="-s -w" -o screener ./cmd/screener && go build -ldflags="-s -w" -o bot ./cmd/bot && systemctl restart screener-long screener-short bot-long bot-short
```

Посмотреть всякие логи
```bash
# 1. Логи закрытых сделок и Time-Stop за последние 4 часа
journalctl -u bot-long -u bot-short --since "4 hours ago" -o cat | grep -E "\[TRADE CLOSED|TIME-STOP|\[SUCCESS\]"

# 2. Текущая сводка по слотам и балансу
journalctl -u bot-long -u bot-short -n 4 -o cat | grep "SUMMARY"
```

Уведомления у выходах из позиции
```bash
journalctl -u bot-long -u bot-short -n 200 -o cat | grep -E "\[TRADE CLOSED|REST RESTORE\]"
```

### Снова логи по торговле
Выгрузка всех сделок и событий (SL, TP, Time-Stop)
Основной дамп торговых событий за последние 6 часов (открытия, закрытия, причины выходов):
```bash
journalctl -u bot-long -u bot-short --since "6 hours ago" -o cat | grep -E "\[SUCCESS\]|\[TRADE CLOSED|TIME-STOP|\[POS MONITOR\]"
```

Сводка по балансу, активным слотам и uPnL
Проверка текущего состояния депо и загрузки слотов $2/2$:
```bash
journalctl -u bot-long -u bot-short -n 10 -o cat | grep "SUMMARY"
```

Проверка скрытых ошибок API и проскальзываний
Дамп сетевых отвалов, ошибок гидратации лота/нотионала и отвергнутых ордеров:
```bash
journalctl -u bot-long -u bot-short --since "6 hours ago" -o cat | grep -E "\[ERROR\]|\[WARN\]|rejected|failed"
```

Комплексный пайплайн (Всё в один клик)
Если хочешь снять полную картину одним запуском:
```bash
echo "=== SUMMARY ===" && journalctl -u bot-long -u bot-short -n 4 -o cat | grep "SUMMARY" && \
echo -e "\n=== TRADES & STOPS (LAST 6H) ===" && journalctl -u bot-long -u bot-short --since "6 hours ago" -o cat | grep -E "\[SUCCESS\]|\[TRADE CLOSED|TIME-STOP" && \
echo -e "\n=== ERRORS ===" && journalctl -u bot-long -u bot-short --since "6 hours ago" -o cat | grep -E "\[ERROR\]|rejected"
```

## Deploy
```bash
deploy
```