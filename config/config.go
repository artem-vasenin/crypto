package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config содержит все настройки скриннера.
type Config struct {
	Bybit       BybitConfig    `json:"bybit"`
	Filters     FilterConfig   `json:"filters"`
	Analysis    AnalysisConfig `json:"analysis"`
	Concurrency int            `json:"concurrency"`
	HTTPTimeout time.Duration  `json:"-"`
	RunTimeout  time.Duration  `json:"-"`
	MaxRetries  int            `json:"-"`
	RetryDelay  time.Duration  `json:"-"`
	Output      OutputConfig   `json:"output"`
}

type BybitConfig struct {
	BaseURL string `json:"base_url"`
}

// FilterConfig разделяет предварительный пул и финальное число кандидатов.
// max_grid_spread_pct применяется только к Grid-стратегиям: обычные Long/Short
// не должны терять монеты только из-за требований сеточного бота к ликвидности.
type FilterConfig struct {
	MaxPrice            float64 `json:"max_price"`
	MinTurnover24h      float64 `json:"min_turnover_24h"`
	PreselectCandidates int     `json:"preselect_candidates"`
	TopCandidates       int     `json:"top_candidates"`
	MaxGridSpreadPct    float64 `json:"max_grid_spread_pct"`
}

type AnalysisConfig struct {
	KlineLimit15m     int `json:"kline_limit_15m"`
	KlineLimit1h      int `json:"kline_limit_1h"`
	KlineLimit4h      int `json:"kline_limit_4h"`
	OpenInterestLimit int `json:"open_interest_limit"`
	FundingLimit      int `json:"funding_limit"`
	OrderBookLimit    int `json:"order_book_limit"`
}

type OutputConfig struct {
	File string `json:"file"`
}

type rawConfig struct {
	Bybit       BybitConfig    `json:"bybit"`
	Filters     FilterConfig   `json:"filters"`
	Analysis    AnalysisConfig `json:"analysis"`
	Concurrency int            `json:"concurrency"`
	HTTPTimeout string         `json:"http_timeout"`
	RunTimeout  string         `json:"run_timeout"`
	MaxRetries  int            `json:"max_retries"`
	RetryDelay  string         `json:"retry_delay"`
	Output      OutputConfig   `json:"output"`
}

// Load читает JSON, преобразует duration и проверяет критичные значения.
// Ошибка конфигурации должна обнаруживаться до первого обращения к Bybit.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var r rawConfig
	if err := json.Unmarshal(b, &r); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	h, err := time.ParseDuration(r.HTTPTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("http_timeout: %w", err)
	}
	run, err := time.ParseDuration(r.RunTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("run_timeout: %w", err)
	}
	delay, err := time.ParseDuration(r.RetryDelay)
	if err != nil {
		return Config{}, fmt.Errorf("retry_delay: %w", err)
	}
	if r.Bybit.BaseURL == "" || r.Filters.MaxPrice <= 0 || r.Filters.MinTurnover24h <= 0 ||
		r.Filters.PreselectCandidates <= 0 || r.Filters.TopCandidates <= 0 ||
		r.Filters.PreselectCandidates < r.Filters.TopCandidates || r.Filters.MaxGridSpreadPct <= 0 ||
		r.Concurrency <= 0 {
		return Config{}, fmt.Errorf("invalid configuration values")
	}
	if r.Analysis.KlineLimit15m < 20 || r.Analysis.KlineLimit1h < 20 || r.Analysis.KlineLimit4h < 20 ||
		r.Analysis.OpenInterestLimit < 2 || r.Analysis.FundingLimit < 1 ||
		r.Analysis.OrderBookLimit < 1 {
		return Config{}, fmt.Errorf("invalid analysis limits")
	}
	return Config{
		Bybit:       r.Bybit,
		Filters:     r.Filters,
		Analysis:    r.Analysis,
		Concurrency: r.Concurrency,
		HTTPTimeout: h,
		RunTimeout:  run,
		MaxRetries:  r.MaxRetries,
		RetryDelay:  delay,
		Output:      r.Output,
	}, nil
}
