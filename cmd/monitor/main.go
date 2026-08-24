package main

import (
	"fmt"
	"log"

	"bybit_monitor/internal/bybit"
	"bybit_monitor/internal/config"
	"bybit_monitor/internal/monitor"
)

func main() {
	cfg, err := config.Load("configs/config.json")
	if err != nil {
		log.Fatal(err)
	}

	client := bybit.NewClient(cfg.ByBit)

	positions, err := client.GetPositions()
	if err != nil {
		log.Fatal(err)
	}

	mon := monitor.New()

	mon.UpdatePositions(positions)

	for _, position := range mon.Positions() {
		fmt.Printf(
			"%s %s %sx PnL=%s\n",
			position.Symbol,
			position.Side,
			position.Leverage,
			position.UnrealisedPnl,
		)
	}

	wsClient, err := bybit.NewWebSocketClient(
		cfg.ByBit.PrivateWebSocketURL,
	)
	if err != nil {
		log.Fatal(err)
	}

	defer wsClient.Close()

	fmt.Println("WebSocket connected")

	err = wsClient.Authenticate(
		cfg.ByBit.APIKey,
		cfg.ByBit.Secret,
	)
	if err != nil {
		log.Fatal(err)
	}

	message, err := wsClient.ReadMessage()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(message))

	err = wsClient.Subscribe("position")
	if err != nil {
		log.Fatal(err)
	}

	message, err = wsClient.ReadMessage()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(message))

	wsClient.StartHeartbeat()

	messages := make(chan []byte)

	go func() {
		defer close(messages)

		for {
			message, err := wsClient.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
				return
			}

			messages <- message
		}
	}()

	for message := range messages {
		wsMessage, err := bybit.ParseWebSocketMessage(message)
		if err != nil {
			log.Println("Parse WebSocket message error:", err)
			continue
		}

		if wsMessage.Topic != "position" {
			continue
		}

		fmt.Println()
		fmt.Println("========== RAW WS ==========")
		fmt.Println(string(message))
		fmt.Println("=============================")

		for _, position := range wsMessage.Data {
			converted := position.ToPosition()

			fmt.Println()
			fmt.Println("========== PARSED POSITION ==========")
			fmt.Printf("%+v\n", converted)
			fmt.Println("=====================================")

			event := mon.UpdatePosition(converted)

			fmt.Println()
			fmt.Println("========== POSITION EVENT ==========")
			fmt.Printf("Type: %s\n", event.Type)
			fmt.Printf("Symbol: %s\n", converted.Symbol)
			fmt.Printf("Side: %s\n", converted.Side)

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
	}
}
