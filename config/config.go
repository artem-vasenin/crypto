// config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Bybit struct {
		BaseURL string `json:"base_url"`
	} `json:"bybit"`
	Filters struct {
		MaxPrice            float64 `json:"max_price"`
		MinTurnover24h      float64 `json:"min_turnover_24h"`
		PreselectCandidates int     `json:"preselect_candidates"`
		TopCandidates       int     `json:"top_candidates"`
		MaxGridSpreadPct    float64 `json:"max_grid_spread_pct"`
	} `json:"filters"`
	Analysis struct {
		KlineLimit15m     int `json:"kline_limit_15m"`
		KlineLimit1h      int `json:"kline_limit_1h"`
		KlineLimit4h      int `json:"kline_limit_4h"`
		OpenInterestLimit int `json:"open_interest_limit"`
		FundingLimit      int `json:"funding_limit"`
		OrderBookLimit    int `json:"order_book_limit"`
	} `json:"analysis"`
	Execution struct {
		Testnet           bool    `json:"testnet"`
		MaxLeverage       int     `json:"max_leverage"`
		MarginPerTradeUSD float64 `json:"margin_per_trade_usd"`
		MinScore          float64 `json:"min_score"`
		TrailingPct       float64 `json:"trailing_pct"`
		CheckInterval     string  `json:"check_interval"`
	} `json:"execution"`
	Concurrency int           `json:"concurrency"`
	HTTPTimeout time.Duration `json:"-"`
	RunTimeout  time.Duration `json:"-"`
	MaxRetries  int           `json:"max_retries"`
	RetryDelay  time.Duration `json:"-"`
	Output      struct {
		File string `json:"file"`
	} `json:"output"`
}

type rawConfig struct {
	Bybit struct {
		BaseURL string `json:"base_url"`
	} `json:"bybit"`
	Filters struct {
		MaxPrice            float64 `json:"max_price"`
		MinTurnover24h      float64 `json:"min_turnover_24h"`
		PreselectCandidates int     `json:"preselect_candidates"`
		TopCandidates       int     `json:"top_candidates"`
		MaxGridSpreadPct    float64 `json:"max_grid_spread_pct"`
	} `json:"filters"`
	Analysis struct {
		KlineLimit15m     int `json:"kline_limit_15m"`
		KlineLimit1h      int `json:"kline_limit_1h"`
		KlineLimit4h      int `json:"kline_limit_4h"`
		OpenInterestLimit int `json:"open_interest_limit"`
		FundingLimit      int `json:"funding_limit"`
		OrderBookLimit    int `json:"order_book_limit"`
	} `json:"analysis"`
	Execution struct {
		Testnet           bool    `json:"testnet"`
		MaxLeverage       int     `json:"max_leverage"`
		MarginPerTradeUSD float64 `json:"margin_per_trade_usd"`
		MinScore          float64 `json:"min_score"`
		TrailingPct       float64 `json:"trailing_pct"`
		CheckInterval     string  `json:"check_interval"`
	} `json:"execution"`
	Concurrency int    `json:"concurrency"`
	HTTPTimeout string `json:"http_timeout"`
	RunTimeout  string `json:"run_timeout"`
	MaxRetries  int    `json:"max_retries"`
	RetryDelay  string `json:"retry_delay"`
	Output      struct {
		File string `json:"file"`
	} `json:"output"`
}

// Load загружает и валидирует конфигурационный файл формата JSON
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file failed: %w", err)
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("unmarshal config json failed: %w", err)
	}

	cfg := Config{
		Bybit:       raw.Bybit,
		Filters:     raw.Filters,
		Analysis:    raw.Analysis,
		Execution:   raw.Execution,
		Concurrency: raw.Concurrency,
		MaxRetries:  raw.MaxRetries,
		Output:      raw.Output,
	}

	// Парсинг временных текстовых интервалов в time.Duration
	cfg.HTTPTimeout, err = time.ParseDuration(raw.HTTPTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("invalid http_timeout: %w", err)
	}

	cfg.RunTimeout, err = time.ParseDuration(raw.RunTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("invalid run_timeout: %w", err)
	}

	cfg.RetryDelay, err = time.ParseDuration(raw.RetryDelay)
	if err != nil {
		return Config{}, fmt.Errorf("invalid retry_delay: %w", err)
	}

	// Дефолтные значения и строгая валидация критичных границ
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	if cfg.Filters.PreselectCandidates <= 0 {
		cfg.Filters.PreselectCandidates = 60
	}
	if cfg.Filters.TopCandidates <= 0 {
		cfg.Filters.TopCandidates = 20
	}

	return cfg, nil
}
