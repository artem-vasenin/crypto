package main

import (
	"bybit_monitor/internal/bybit"
	"bybit_monitor/internal/config"
	"fmt"
)

func main() {
	conf, err := config.Load("configs/config.json")
	if err != nil {
		panic(err)
	}
	fmt.Println(conf)
	client := bybit.NewClient(conf.ByBit)
	body, err := client.GetPositions()
	if err != nil {
		panic(err)
	}
	fmt.Println(body)
}
