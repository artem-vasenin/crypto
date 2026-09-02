// cmd/bot/main.go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"universal-bybit-screener/config"
	"universal-bybit-screener/internal/execution"
	"universal-bybit-screener/models"

	"github.com/joho/godotenv"
)

func main() {
	strategyName := flag.String("strategy", "long", "target strategy for automated trade execution")
	configPath := flag.String("config", "configs/config.json", "path to configuration file")
	inputFile := flag.String("input", "long-screening.json", "path to input screening result JSON")
	flag.Parse()

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)

	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] .env file not found, falling back to system environment variables")
	}

	log.Println("[INFO] Initializing Execution Engine Service...")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Configuration load failed: %v", err)
	}

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
		ApiKey:             apiKey,
		ApiSecret:          apiSecret,
		Testnet:            cfg.Execution.Testnet,
		MaxLeverage:        cfg.Execution.MaxLeverage,
		MarginPerTradeUSD:  cfg.Execution.MarginPerTradeUSD,
		MaxTotalMarginUSD:  cfg.Execution.MaxTotalMarginUSD,
		MaxActivePositions: cfg.Execution.MaxActivePositions,
		MinScore:           cfg.Execution.MinScore,
		TrailingPct:        cfg.Execution.TrailingPct,
		CheckInterval:      checkInterval,
	}

	engine := execution.NewEngine(botCfg, *strategyName)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := engine.InitWebSocket(ctx); err != nil {
		log.Printf("[WARN] WebSocket initialization warning: %v", err)
	}

	// Первичная разовая вычитка баланса и позиций при холодно-стартовом вызове
	if err := engine.RefreshBalance(ctx); err != nil {
		log.Printf("[ERROR] Initial balance refresh failed: %v", err)
	}
	engine.LogActivePositions(ctx)

	log.Printf("[INFO] Engine Active. Strategy: %s | Margin/Trade: $%.2f | MaxMargin: $%.2f | MaxPos: %d | Leverage: x%d | Testnet: %v",
		*strategyName, botCfg.MarginPerTradeUSD, botCfg.MaxTotalMarginUSD, botCfg.MaxActivePositions, botCfg.MaxLeverage, botCfg.Testnet)

	ticker := time.NewTicker(botCfg.CheckInterval)
	defer ticker.Stop()

	processIteration(ctx, engine, *inputFile, *strategyName, cfg.Concurrency)

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Shutdown signal received. Closing connections...")
			return
		case <-ticker.C:
			processIteration(ctx, engine, *inputFile, *strategyName, cfg.Concurrency)
		}
	}
}

func processIteration(ctx context.Context, engine *execution.Engine, filePath, targetStrategy string, concurrency int) {
	if ctx.Err() != nil {
		return
	}

	// Вызов RefreshBalance здесь УБРАН: обновление идет исключительно по WebSocket Push от Bybit
	engine.LogActivePositions(ctx)

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

	log.Printf("[ENGINE] Processing snapshot generated at %s. Candidates: %d",
		result.GeneratedAt.Format(time.RFC3339), len(result.Candidates))

	if concurrency <= 0 {
		concurrency = 5
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, cand := range result.Candidates {
		if ctx.Err() != nil {
			break
		}

		cand := cand
		wg.Add(1)

		go func(c models.Candidate) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			engine.UpdateTrailingStops(ctx, c.Symbol, c.Market.Price)

			if err := engine.ProcessCandidate(ctx, c, targetStrategy); err != nil {
				if ctx.Err() == nil {
					log.Printf("[ERROR] Failed to process candidate %s: %v", c.Symbol, err)
				}
			}
		}(cand)
	}

	wg.Wait()
}
