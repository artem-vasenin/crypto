// cmd/screener/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"universal-bybit-screener/config"
	"universal-bybit-screener/internal/analysis"
	"universal-bybit-screener/internal/bybit"
	"universal-bybit-screener/internal/strategies"
	"universal-bybit-screener/output"
)

func main() {
	strategyName := flag.String("strategy", "long", "short-grid, short, long-grid, long, neutral-grid")
	configPath := flag.String("config", "configs/config.json", "path to configuration file")
	interval := flag.Duration("interval", 3*time.Minute, "screening execution interval")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	log.Println("[INFO] Initializing Screener Daemon Service...")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Configuration load failed: %v", err)
	}

	strategy, err := strategies.New(*strategyName)
	if err != nil {
		log.Fatalf("[FATAL] Strategy creation failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := bybit.NewClient(bybit.ClientConfig{
		BaseURL:     cfg.Bybit.BaseURL,
		HTTPTimeout: cfg.HTTPTimeout,
		MaxRetries:  cfg.MaxRetries,
		RetryDelay:  cfg.RetryDelay,
	})

	service := analysis.NewService(client, cfg, strategy)
	outFileName := fmt.Sprintf("%s-%s", strategy.Name(), cfg.Output.File)

	log.Printf("[INFO] Daemon active. Primary Strategy: %s | Execution Interval: %s", strategy.Name(), *interval)

	// Первичный запуск
	executeScreening(ctx, service, strategy.Name(), outFileName)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Shutdown signal received. Terminating Screener Daemon...")
			return
		case <-ticker.C:
			executeScreening(ctx, service, strategy.Name(), outFileName)
		}
	}
}

func executeScreening(ctx context.Context, service *analysis.Service, strategyName, outFileName string) {
	runCtx, runCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer runCancel()

	start := time.Now()
	log.Printf("[INFO] Executing screening iteration for strategy: %s", strategyName)

	result, err := service.Run(runCtx)
	if err != nil {
		log.Printf("[ERROR] Screening iteration failed: %v", err)
		return
	}

	written, err := output.WriteJSONIfChanged(outFileName, result)
	if err != nil {
		log.Printf("[ERROR] Output write operation failed: %v", err)
		return
	}

	if written {
		log.Printf("[INFO] Cycle finished in %s. Matched Candidates: %d. File %s updated on disk.",
			time.Since(start).Round(time.Millisecond), len(result.Candidates), outFileName)
	} else {
		log.Printf("[INFO] Cycle finished in %s. Candidates matched: %d. Payload unchanged (skipped I/O).",
			time.Since(start).Round(time.Millisecond), len(result.Candidates))
	}
}
