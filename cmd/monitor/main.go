package main

import (
	"fmt"
	"log"

	"bybit_monitor/internal/bybit"
	"bybit_monitor/internal/config"
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

	for _, position := range positions {
		fmt.Printf(
			"%s %s %sx PnL=%s\n",
			position.Symbol,
			position.Side,
			position.Leverage,
			position.UnrealisedPnl,
		)
	}
}
