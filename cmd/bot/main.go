// cmd/bot/main.go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"universal-bybit-screener/config"
	"universal-bybit-screener/internal/execution"
	"universal-bybit-screener/models"
)

func main() {
	strategyName := flag.String("strategy", "long", "target strategy for automated trade execution")
	configPath := flag.String("config", "configs/config.json", "path to configuration file")
	inputFile := flag.String("input", "long-screening.json", "path to input screening result JSON")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	log.Println("[INFO] Initializing Execution Engine Service...")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Configuration load failed: %v", err)
	}

	// Чтение приватных ключей из переменных окружения (безопасность)
	apiKey := os.Getenv("BYBIT_API_KEY")
	apiSecret := os.Getenv("BYBIT_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		log.Fatalf("[FATAL] Environment variables BYBIT_API_KEY and BYBIT_API_SECRET must be set")
	}

	checkInterval, err := time.ParseDuration(cfg.Execution.CheckInterval)
	if err != nil {
		checkInterval = 1 * time.Minute
	}

	botCfg := models.BotConfig{
		ApiKey:            apiKey,
		ApiSecret:         apiSecret,
		Testnet:           cfg.Execution.Testnet,
		MaxLeverage:       cfg.Execution.MaxLeverage,
		MarginPerTradeUSD: cfg.Execution.MarginPerTradeUSD,
		MinScore:          cfg.Execution.MinScore,
		TrailingPct:       cfg.Execution.TrailingPct,
		CheckInterval:     checkInterval,
	}

	engine := execution.NewEngine(botCfg)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("[INFO] Engine Started. Target Strategy: %s | Margin: $%.2f | Leverage: x%d | Testnet: %v",
		*strategyName, botCfg.MarginPerTradeUSD, botCfg.MaxLeverage, botCfg.Testnet)

	ticker := time.NewTicker(botCfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Graceful Shutdown. Terminating Execution Engine...")
			return
		case <-ticker.C:
			processIteration(ctx, engine, *inputFile, *strategyName)
		}
	}
}

func processIteration(ctx context.Context, engine *execution.Engine, filePath, targetStrategy string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[WARN] Failed to read screening JSON file %s: %v", filePath, err)
		return
	}

	var result models.ScreeningResult
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("[ERROR] Failed to unmarshal screening payload: %v", err)
		return
	}

	log.Printf("[ENGINE] Processing screening snapshot generated at %s. Candidates count: %d",
		result.GeneratedAt.Format(time.RFC3339), len(result.Candidates))

	for _, cand := range result.Candidates {
		// Обновление трейлинга активных позиций
		engine.UpdateTrailingStops(ctx, cand.Symbol, cand.Market.Price)

		// Оценка кандидата на открытие ордера
		if err := engine.ProcessCandidate(ctx, cand, targetStrategy); err != nil {
			log.Printf("[ERROR] Failed to process candidate %s: %v", cand.Symbol, err)
		}
	}
}
