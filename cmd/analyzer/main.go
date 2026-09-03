// Приложение принимает символ Bybit USDT Perpetual (например, SOLUSDT),
// получает публичные рыночные данные и формирует один JSON-файл,
// который можно целиком передать ИИ для последующего анализа Long/Short.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"crypto-coin-analyzer/internal/analysis"
	"crypto-coin-analyzer/internal/bybit"
	"crypto-coin-analyzer/internal/output"
)

func main() {
	var (
		symbol = flag.String("symbol", "", "Символ Bybit USDT Perpetual, например SOLUSDT")
		out    = flag.String("out", "", "Путь для JSON; если не указан, JSON выводится в stdout")
		days   = flag.Int("days", 7, "Глубина 1m истории для lead-lag BTC, максимум 7")
		pretty = flag.Bool("pretty", true, "Красиво форматировать JSON")
	)
	flag.Parse()

	if strings.TrimSpace(*symbol) == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -symbol, например: go run ./cmd/analyzer -symbol SOLUSDT")
		os.Exit(2)
	}

	if *days < 1 {
		*days = 1
	}
	if *days > 7 {
		*days = 7
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client := bybit.NewClient(bybit.Config{BaseURL: "https://api.bybit.com", Timeout: 20 * time.Second})
	result, err := analysis.Build(ctx, client, strings.ToUpper(strings.TrimSpace(*symbol)), *days)
	if err != nil {
		log.Printf("ошибка анализа: %v", err)
		os.Exit(1)
	}

	if err := output.WriteJSON(os.Stdout, result, *pretty); err != nil {
		log.Printf("ошибка записи JSON: %v", err)
		os.Exit(1)
	}

	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			log.Fatalf("не удалось создать %s: %v", *out, err)
		}
		defer f.Close()
		if err := output.WriteJSON(f, result, *pretty); err != nil {
			log.Fatalf("не удалось записать %s: %v", *out, err)
		}
		fmt.Fprintf(os.Stderr, "JSON сохранён: %s\n", *out)
	}
}
