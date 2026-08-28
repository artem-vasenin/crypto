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
// Это важно для универсальности: нельзя отбирать рынок только по росту 24h,
// иначе Neutral Grid будет терять хорошие боковые монеты ещё до анализа свечей.
type FilterConfig struct {
	MaxPrice            float64 `json:"max_price"`
	MinTurnover24h      float64 `json:"min_turnover_24h"`
	PreselectCandidates int     `json:"preselect_candidates"`
	TopCandidates       int     `json:"top_candidates"`
}

type AnalysisConfig struct {
	KlineLimit15m     int `json:"kline_limit_15m"`
	KlineLimit1h      int `json:"kline_limit_1h"`
	KlineLimit4h      int `json:"kline_limit_4h"`
	OpenInterestLimit int `json:"open_interest_limit"`
	FundingLimit      int `json:"funding_limit"`
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

// Load читает JSON и сразу проверяет обязательные настройки.
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
	if r.Bybit.BaseURL == "" || r.Filters.MaxPrice <= 0 || r.Filters.MinTurnover24h <= 0 || r.Filters.PreselectCandidates <= 0 || r.Filters.TopCandidates <= 0 || r.Filters.PreselectCandidates < r.Filters.TopCandidates || r.Concurrency <= 0 {
		return Config{}, fmt.Errorf("invalid configuration values")
	}
	if r.Analysis.KlineLimit15m < 20 || r.Analysis.KlineLimit1h < 20 || r.Analysis.KlineLimit4h < 20 {
		return Config{}, fmt.Errorf("kline limits must be at least 20")
	}
	return Config{Bybit: r.Bybit, Filters: r.Filters, Analysis: r.Analysis, Concurrency: r.Concurrency, HTTPTimeout: h, RunTimeout: run, MaxRetries: r.MaxRetries, RetryDelay: delay, Output: r.Output}, nil
}
