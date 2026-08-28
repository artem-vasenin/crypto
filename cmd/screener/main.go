package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"universal-bybit-screener/config"
	"universal-bybit-screener/internal/analysis"
	"universal-bybit-screener/internal/bybit"
	"universal-bybit-screener/internal/strategies"
	"universal-bybit-screener/output"
)

func main() {
	// Защита от panic оставляет консоль открытой на Windows, чтобы ошибку можно было прочитать.
	defer func() {
		if r := recover(); r != nil {
			fmt.Println()
			fmt.Println("========================================")
			fmt.Println("КРИТИЧЕСКАЯ ОШИБКА")
			fmt.Println("========================================")
			fmt.Printf("%v\n", r)
			waitForEnter()
		}
	}()
	strategyName := flag.String("strategy", "", "short-grid, short, long-grid, long, neutral-grid")
	configPath := flag.String("config", "configs/config.json", "configuration file")
	flag.Parse()
	scanner := bufio.NewScanner(os.Stdin)
	if strings.TrimSpace(*strategyName) == "" {
		*strategyName = selectStrategy(scanner)
	}
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("ЗАПУСК СКРИННЕРА")
	fmt.Println("========================================")
	fmt.Printf("Стратегия: %s\n", *strategyName)
	fmt.Println()
	fmt.Println("1/5 Загрузка конфигурации...")
	cfg, err := config.Load(*configPath)
	if err != nil {
		handleError("Ошибка загрузки конфигурации", err, scanner)
		return
	}
	fmt.Println("    Конфигурация загружена.")
	fmt.Println()
	fmt.Println("2/5 Создание стратегии...")
	strategy, err := strategies.New(*strategyName)
	if err != nil {
		handleError("Ошибка создания стратегии", err, scanner)
		return
	}
	fmt.Printf("    Стратегия создана: %s\n\n", strategy.Name())
	fmt.Println("3/5 Создание HTTP-клиента Bybit...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.RunTimeout)
	defer cancel()
	client := bybit.NewClient(bybit.ClientConfig{BaseURL: cfg.Bybit.BaseURL, HTTPTimeout: cfg.HTTPTimeout, MaxRetries: cfg.MaxRetries, RetryDelay: cfg.RetryDelay})
	fmt.Println("    Клиент Bybit создан.")
	fmt.Println()
	fmt.Println("4/5 Запуск анализа...")
	result, err := analysis.NewService(client, cfg, strategy).Run(ctx)
	if err != nil {
		handleError("Ошибка во время анализа", err, scanner)
		return
	}
	fmt.Printf("    Анализ успешно завершён. Кандидатов: %d\n\n", len(result.Candidates))
	fmt.Println("5/5 Сохранение JSON...")
	outFileName := strategy.Name() + "-" + cfg.Output.File
	if err := output.WriteJSON(outFileName, result); err != nil {
		handleError("Ошибка сохранения JSON", err, scanner)
		return
	}
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("SCREENING COMPLETED")
	fmt.Println("========================================")
	fmt.Printf("Strategy:   %s\nCandidates: %d\nJSON:       %s\n", strategy.Name(), len(result.Candidates), outFileName)
	fmt.Println("========================================")
	fmt.Println()
	waitForEnterWithScanner(scanner)
}

// selectStrategy показывает меню всех пяти аналитических режимов.
func selectStrategy(scanner *bufio.Scanner) string {
	for {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Println("BYBIT CRYPTO SCREENER")
		fmt.Println("========================================")
		fmt.Println()
		fmt.Println("Выберите режим анализа:")
		fmt.Println()
		fmt.Println("  1. Short Grid")
		fmt.Println("  2. Short")
		fmt.Println("  3. Long Grid")
		fmt.Println("  4. Long")
		fmt.Println("  5. Neutral Grid")
		fmt.Println()
		fmt.Print("Введите номер режима: ")
		if !scanner.Scan() {
			fmt.Println("Не удалось прочитать выбор стратегии.")
			waitForEnterWithScanner(scanner)
			os.Exit(1)
		}
		choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil {
			fmt.Println("Ошибка: необходимо ввести число от 1 до 5.")
			continue
		}
		switch choice {
		case 1:
			return "short-grid"
		case 2:
			return "short"
		case 3:
			return "long-grid"
		case 4:
			return "long"
		case 5:
			return "neutral-grid"
		default:
			fmt.Println("Ошибка: такого режима нет. Введите число от 1 до 5.")
		}
	}
}

// handleError выводит ошибку и ждёт Enter вместо log.Fatal/os.Exit.
func handleError(message string, err error, scanner *bufio.Scanner) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("ОШИБКА")
	fmt.Println("========================================")
	fmt.Println(message)
	fmt.Printf("Подробности: %v\n", err)
	fmt.Println("========================================")
	waitForEnterWithScanner(scanner)
}
func waitForEnterWithScanner(scanner *bufio.Scanner) {
	fmt.Print("Нажмите Enter для выхода...")
	scanner.Scan()
	fmt.Println()
}
func waitForEnter() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Нажмите Enter для выхода...")
	scanner.Scan()
	fmt.Println()
}
