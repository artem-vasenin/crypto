package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bybit_monitor/internal/bybit"
	"bybit_monitor/internal/config"
	"bybit_monitor/internal/monitor"
)

func main() {
	// ------------------------------------------------------------
	// 1. Загружаем конфигурацию.
	//
	// Путь можно передать через переменную окружения CONFIG_PATH.
	//
	// Если переменная не задана, используем стандартный:
	//
	//     configs/config.json
	//
	// Это удобно, потому что можно запускать:
	//
	//     CONFIG_PATH=config.testnet.json go run ./cmd/monitor
	//
	// или просто:
	//
	//     go run ./cmd/monitor
	// ------------------------------------------------------------

	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		configPath = "configs/config.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal("load config:", err)
	}

	log.Printf(
		"starting Bybit monitor: REST=%s WS=%s",
		cfg.ByBit.BaseURL,
		cfg.ByBit.PrivateWebSocketURL,
	)

	// ------------------------------------------------------------
	// 2. Создаём REST-клиент.
	//
	// REST нужен нам для первоначальной синхронизации состояния.
	//
	// WebSocket сообщает только об изменениях.
	// Поэтому правильная схема:
	//
	//     REST -> получаем текущее состояние
	//     WS   -> дальше получаем изменения
	// ------------------------------------------------------------

	client := bybit.NewClient(cfg.ByBit)

	positions, err := client.GetPositions()
	if err != nil {
		log.Fatal("get positions:", err)
	}

	// ------------------------------------------------------------
	// 3. Создаём Monitor.
	//
	// Monitor не знает ничего про HTTP и WebSocket.
	//
	// Его задача намного проще:
	//
	//     хранить состояние
	//     сравнивать старое состояние с новым
	//     определять opened / updated / closed
	// ------------------------------------------------------------

	mon := monitor.New()

	mon.UpdatePositions(positions)

	fmt.Println("========== INITIAL POSITIONS ==========")

	for _, position := range mon.Positions() {
		fmt.Printf(
			"%s %s %sx PnL=%s\n",
			position.Symbol,
			position.Side,
			position.Leverage,
			position.UnrealisedPnl,
		)
	}

	fmt.Println("=======================================")

	// ------------------------------------------------------------
	// 4. Создаём context.
	//
	// Context нужен для корректного завершения приложения.
	//
	// Например:
	//
	//     Ctrl+C
	//
	// должен не просто убить процесс, а дать goroutine возможность
	// аккуратно завершиться и закрыть WebSocket.
	// ------------------------------------------------------------

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer cancel()

	// ------------------------------------------------------------
	// 5. Запускаем WebSocket.
	//
	// В production-версии WebSocket-клиент сам:
	//
	//     подключается
	//     авторизуется
	//     подписывается
	//     поддерживает heartbeat
	//     переподключается после разрыва
	//
	// Main здесь занимается только обработкой сообщений.
	// ------------------------------------------------------------

	wsClient := bybit.NewWebSocketClient(cfg.ByBit)

	defer wsClient.Close()

	err = wsClient.Run(
		ctx,
		[]string{"position"},
		func(message []byte) {
			handleWebSocketMessage(mon, message)
		},
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("websocket:", err)
	}

	fmt.Println("monitor stopped")

	// Небольшая пауза нужна только для аккуратного завершения логов.
	time.Sleep(100 * time.Millisecond)
}

// handleWebSocketMessage занимается только преобразованием сообщения
// WebSocket и передачей позиции в Monitor.
//
// Это специально отдельная функция.
//
// main() тогда остаётся относительно чистым и читаемым.
func handleWebSocketMessage(
	mon *monitor.Monitor,
	message []byte,
) {
	wsMessage, err := bybit.ParseWebSocketMessage(message)
	if err != nil {
		log.Println("parse websocket message:", err)
		return
	}

	// Нас интересует только private topic "position".
	if wsMessage.Topic != "position" {
		return
	}

	for _, wsPosition := range wsMessage.Data {
		position := wsPosition.ToPosition()

		event := mon.UpdatePosition(position)

		printPositionEvent(event)
	}
}

// printPositionEvent отвечает только за отображение события.
//
// В будущем эту функцию можно заменить, например, на:
//
//	Telegram notifier
//	database writer
//	REST API
//	GUI
//
// При этом Monitor менять практически не придётся.
func printPositionEvent(event monitor.PositionEvent) {
	fmt.Println()
	fmt.Println("========== POSITION EVENT ==========")

	fmt.Printf("Type: %s\n", event.Type)

	if event.Current != nil {
		fmt.Printf(
			"Symbol: %s\n",
			event.Current.Symbol,
		)

		fmt.Printf(
			"Side: %s\n",
			event.Current.Side,
		)
	} else if event.Previous != nil {
		fmt.Printf(
			"Symbol: %s\n",
			event.Previous.Symbol,
		)

		fmt.Printf(
			"Side: %s\n",
			event.Previous.Side,
		)
	}

	if len(event.Changes) > 0 {
		fmt.Println("Changes:")

		for _, change := range event.Changes {
			fmt.Printf(
				"  %s: %s → %s\n",
				change.Field,
				change.From,
				change.To,
			)
		}
	}

	fmt.Println("====================================")
}
