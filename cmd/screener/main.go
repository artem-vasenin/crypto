// cmd/screener/main.go
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"

	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"universal-bybit-screener/config"
	"universal-bybit-screener/internal/analysis"
	"universal-bybit-screener/internal/bybit"
	"universal-bybit-screener/internal/strategies"
	"universal-bybit-screener/output"
)

func main() {
	strategyName := flag.String("strategy", "", "short-grid, short, long-grid, long, neutral-grid")
	configPath := flag.String("config", "configs/config.json", "configuration file")
	interval := flag.Duration("interval", 3*time.Minute, "screening execution interval")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	log.Println("[INFO] Initializing Screener Daemon...")

	if strings.TrimSpace(*strategyName) == "" {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("Select Strategy:")
		fmt.Println("1. short-grid\n2. short\n3. long-grid\n4. long\n5. neutral-grid")
		fmt.Print("Choice (1-5): ")
		if scanner.Scan() {
			switch strings.TrimSpace(scanner.Text()) {
			case "1":
				*strategyName = "short-grid"
			case "2":
				*strategyName = "short"
			case "3":
				*strategyName = "long-grid"
			case "4":
				*strategyName = "long"
			case "5":
				*strategyName = "neutral-grid"
			}
		}
	}

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

	log.Printf("[INFO] Daemon started. Strategy: %s, Interval: %s", strategy.Name(), *interval)

	// Первичный запуск
	executeScreening(ctx, service, strategy.Name(), outFileName)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Shutdown signal received. Exiting daemon...")
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
	log.Printf("[INFO] Starting screening cycle for strategy: %s", strategyName)

	result, err := service.Run(runCtx)
	if err != nil {
		log.Printf("[ERROR] Screening cycle failed: %v", err)
		return
	}

	if err := output.WriteJSON(outFileName, result); err != nil {
		log.Printf("[ERROR] Output write failed: %v", err)
		return
	}

	log.Printf("[INFO] Cycle completed in %s. Candidates matched: %d. File: %s",
		time.Since(start).Round(time.Millisecond), len(result.Candidates), outFileName)
}
