package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Bybit    BybitConfig    `json:"bybit"`
	Filters  FilterConfig   `json:"filters"`
	Analysis AnalysisConfig `json:"analysis"`
	Output   OutputConfig   `json:"output"`
}

type BybitConfig struct {
	BaseURL  string `json:"base_url"`
	Category string `json:"category"`
}

type FilterConfig struct {
	MaxPrice            float64 `json:"max_price"`
	MinTurnover24h      float64 `json:"min_turnover_24h"`
	TopCandidates       int     `json:"top_candidates"`
	PrefilterByChange24 bool    `json:"prefilter_by_change_24h"`
}

type AnalysisConfig struct {
	KlineLimit15m      int       `json:"kline_limit_15m"`
	KlineLimit1h       int       `json:"kline_limit_1h"`
	KlineLimit4h       int       `json:"kline_limit_4h"`
	KlineLimit5m       int       `json:"kline_limit_5m"`
	PivotWindow        int       `json:"pivot_window"`
	VolumeBaselineBars int       `json:"volume_baseline_bars"`
	OrderbookLimit     int       `json:"orderbook_limit"`
	OrderbookDepthPct  []float64 `json:"orderbook_depth_pct"`
	RecentTradesLimit  int       `json:"recent_trades_limit"`
	OIInterval         string    `json:"oi_interval"`
	OILimit            int       `json:"oi_limit"`
	FundingLimit       int       `json:"funding_limit"`
	Concurrency        int       `json:"concurrency"`
}

type OutputConfig struct {
	File string `json:"file"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if cfg.Bybit.BaseURL == "" {
		cfg.Bybit.BaseURL = "https://api.bybit.com"
	}
	if cfg.Bybit.Category == "" {
		cfg.Bybit.Category = "linear"
	}
	if cfg.Analysis.Concurrency < 1 {
		cfg.Analysis.Concurrency = 1
	}
	if cfg.Filters.TopCandidates < 1 {
		cfg.Filters.TopCandidates = 20
	}

	return cfg, nil
}
