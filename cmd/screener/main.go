package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"bybit-screener/internal/bybit"
	"bybit-screener/internal/config"
	"bybit-screener/internal/output"
	"bybit-screener/internal/screener"
)

// Стратегии, поддерживаемые скринером.
//
// Мы используем строковые значения, потому что именно они
// передаются через командную строку и записываются в JSON.
const (
	strategyShortGrid = "short-grid"
	strategyLongGrid  = "long-grid"
	strategyShort     = "short"
	strategyLong      = "long"
)

func main() {
	// Путь к конфигурационному файлу.
	//
	// По умолчанию используется:
	// configs/config.json
	configPath := flag.String(
		"config",
		"configs/config.json",
		"path to config file",
	)

	// Основной флаг запуска.
	//
	// Например:
	//
	// go run ./cmd/screener -strategy short-grid
	//
	// Если флаг не указан, по умолчанию запускается short-grid.
	strategy := flag.String(
		"strategy",
		strategyShortGrid,
		"screening strategy: short-grid, long-grid, short, long",
	)

	// Переопределяем стандартный текст справки flag.
	//
	// Это необязательно для работы программы, но делает
	// запуск намного понятнее.
	flag.Usage = func() {
		fmt.Println("Bybit Screener")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go run ./cmd/screener [flags]")
		fmt.Println()
		fmt.Println("Flags:")

		flag.PrintDefaults()

		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  Short Grid:")
		fmt.Println("    go run ./cmd/screener -strategy short-grid")
		fmt.Println()
		fmt.Println("  Long Grid:")
		fmt.Println("    go run ./cmd/screener -strategy long-grid")
		fmt.Println()
		fmt.Println("  Short:")
		fmt.Println("    go run ./cmd/screener -strategy short")
		fmt.Println()
		fmt.Println("  Long:")
		fmt.Println("    go run ./cmd/screener -strategy long")
		fmt.Println()
		fmt.Println("  Custom config:")
		fmt.Println("    go run ./cmd/screener -config configs/config.json -strategy short-grid")
	}

	flag.Parse()

	// Проверяем стратегию сразу после разбора аргументов.
	//
	// Это важно: если пользователь случайно напишет:
	//
	// -strategy shrot-grid
	//
	// программа не должна начинать обращаться к Bybit.
	if !isValidStrategy(*strategy) {
		log.Fatalf(
			"unknown strategy %q; allowed values: %s, %s, %s, %s",
			*strategy,
			strategyShortGrid,
			strategyLongGrid,
			strategyShort,
			strategyLong,
		)
	}

	// Загружаем конфигурацию.
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	// Ограничиваем максимальное время работы одного запуска.
	//
	// При большом количестве кандидатов screener делает
	// достаточно много запросов к Bybit, поэтому timeout
	// защищает программу от зависания навсегда.
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Minute,
	)
	defer cancel()

	// Создаём публичный Bybit API client.
	//
	// Для получения market data API key не требуется.
	client := bybit.NewClient(cfg.Bybit.BaseURL)

	// Создаём screener и передаём ему конфигурацию.
	sc := &screener.Screener{
		Client: client,
		Config: cfg,
	}

	// Запускаем анализ именно выбранной стратегии.
	//
	// strategy одновременно используется:
	//
	// 1. для выбора нужного score при сортировке кандидатов;
	// 2. для записи primary_strategy в итоговый JSON.
	result, err := sc.Run(ctx, *strategy)
	if err != nil {
		log.Fatal(err)
	}

	// Формируем имя выходного JSON-файла.
	//
	// Например:
	//
	// short-grid-screening.json
	// long-grid-screening.json
	// short-screening.json
	// long-screening.json
	//
	// Это позволяет запускать несколько стратегий подряд,
	// не перезаписывая предыдущий результат.
	outputFile := fmt.Sprintf(
		"%s-%s",
		*strategy,
		cfg.Output.File,
	)

	// Сохраняем результат в JSON-файл.
	if err := output.WriteJSON(outputFile, result); err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"strategy=%s: generated %d candidates -> %s\n",
		*strategy,
		len(result.Candidates),
		outputFile,
	)
}

// isValidStrategy проверяет, поддерживает ли screener
// переданную через командную строку стратегию.
func isValidStrategy(strategy string) bool {
	switch strategy {
	case strategyShortGrid,
		strategyLongGrid,
		strategyShort,
		strategyLong:
		return true
	default:
		return false
	}
}
