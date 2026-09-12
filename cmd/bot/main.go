package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"universal-bybit-screener/config"
	"universal-bybit-screener/internal/execution"
	"universal-bybit-screener/models"

	"github.com/joho/godotenv"
)

func main() {
	strategyName := flag.String("strategy", "long", "long or short")
	configPath := flag.String("config", "configs/config.json", "path to configuration file")
	inputFile := flag.String("input", "long-screening.json", "path to input screening result JSON")
	flag.Parse()

	if *strategyName != "long" && *strategyName != "short" {
		log.Fatalf("[FATAL] Bot supports only long and short execution. Grid strategies are screening-only.")
	}

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)

	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] .env file not found, using system environment variables")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Configuration load failed: %v", err)
	}

	apiKey := os.Getenv("BYBIT_API_KEY")
	apiSecret := os.Getenv("BYBIT_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		log.Fatalf("[FATAL] BYBIT_API_KEY and BYBIT_API_SECRET must be set")
	}

	checkInterval, err := time.ParseDuration(cfg.Execution.CheckInterval)
	if err != nil {
		checkInterval = time.Minute
	}
	pendingTimeout, err := time.ParseDuration(cfg.Execution.PendingOrderTimeout)
	if err != nil {
		pendingTimeout = 5 * time.Minute
	}
	maxScreeningAge, err := time.ParseDuration(cfg.Execution.MaxScreeningAge)
	if err != nil {
		maxScreeningAge = 3 * time.Minute
	}

	botCfg := models.BotConfig{
		ApiKey:              apiKey,
		ApiSecret:           apiSecret,
		Testnet:             cfg.Execution.Testnet,
		MaxLeverage:         cfg.Execution.MaxLeverage,
		MarginPerTradeUSD:   cfg.Execution.MarginPerTradeUSD,
		MaxTotalMarginUSD:   cfg.Execution.MaxTotalMarginUSD,
		MaxActivePositions:  cfg.Execution.MaxActivePositions,
		TrailingPct:         cfg.Execution.TrailingPct,
		CheckInterval:       checkInterval,
		PendingOrderTimeout: pendingTimeout,
		MakerFeeRate:        cfg.Execution.MakerFeeRate,
		TakerFeeRate:        cfg.Execution.TakerFeeRate,
		ExtraCostPct:        cfg.Execution.ExtraCostPct,
		MaxStopLossPct:      cfg.Execution.MaxStopLossPct,
		MinNetProfitPct:     cfg.Execution.MinNetProfitPct,
		MaxScreeningAge:     maxScreeningAge,
	}

	engine := execution.NewEngine(botCfg, *strategyName)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := engine.InitWebSocket(ctx); err != nil {
		log.Fatalf("[FATAL] WebSocket initialization failed: %v", err)
	}
	if err := engine.RefreshState(ctx); err != nil {
		log.Fatalf("[FATAL] Initial account state refresh failed: %v", err)
	}
	engine.LogActivePositions(ctx)

	log.Printf("[INFO] Bot active | strategy=%s target_side=%s margin=$%.2f max_margin=$%.2f max_positions=%d leverage<=x%d testnet=%v",
		*strategyName,
		map[bool]string{true: "Buy", false: "Sell"}[strings.EqualFold(*strategyName, "long")],
		botCfg.MarginPerTradeUSD,
		botCfg.MaxTotalMarginUSD,
		botCfg.MaxActivePositions,
		botCfg.MaxLeverage,
		botCfg.Testnet,
	)

	ticker := time.NewTicker(botCfg.CheckInterval)
	defer ticker.Stop()

	processIteration(ctx, engine, *inputFile, *strategyName, cfg.Concurrency)

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Shutdown signal received")
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

	engine.CleanupPendingOrders(ctx)

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[WARN] screening file %s: %v", filePath, err)
		return
	}

	var result models.ScreeningResult
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("[ERROR] screening JSON: %v", err)
		return
	}

	age := time.Since(result.GeneratedAt)
	log.Printf("[ENGINE] snapshot=%s age=%s candidates=%d",
		result.GeneratedAt.Format(time.RFC3339), age.Round(time.Second), len(result.Candidates))
	if age < 0 || age > engine.MaxScreeningAge() {
		log.Printf("[WARN] screening snapshot is stale: age=%s max=%s", age.Round(time.Second), engine.MaxScreeningAge())
		return
	}

	if concurrency <= 0 {
		concurrency = 4
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, candidate := range result.Candidates {
		candidate := candidate
		wg.Add(1)

		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			engine.UpdateTrailingStops(ctx, candidate.Symbol, candidate.Market.Price)

			if err := engine.ProcessCandidate(ctx, candidate, targetStrategy); err != nil && ctx.Err() == nil {
				log.Printf("[WARN] candidate %s: %v", candidate.Symbol, err)
			}
		}()
	}

	wg.Wait()
	engine.LogActivePositions(ctx)
}
