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
}
