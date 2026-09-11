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
		KlineLimit5m      int `json:"kline_limit_5m"`
		KlineLimit15m     int `json:"kline_limit_15m"`
		KlineLimit30m     int `json:"kline_limit_30m"`
		KlineLimit1h      int `json:"kline_limit_1h"`
		KlineLimit4h      int `json:"kline_limit_4h"`
		OpenInterestLimit int `json:"open_interest_limit"`
		FundingLimit      int `json:"funding_limit"`
		OrderBookLimit    int `json:"order_book_limit"`
		MaxDataAgeSeconds int `json:"max_data_age_seconds"`
	} `json:"analysis"`

	Execution struct {
		Testnet             bool    `json:"testnet"`
		MaxLeverage         int     `json:"max_leverage"`
		MarginPerTradeUSD   float64 `json:"margin_per_trade_usd"`
		MaxTotalMarginUSD   float64 `json:"max_total_margin_usd"`
		MaxActivePositions  int     `json:"max_active_positions"`
		MinScore            float64 `json:"min_score"`
		TrailingPct         float64 `json:"trailing_pct"`
		CheckInterval       string  `json:"check_interval"`
		PendingOrderTimeout string  `json:"pending_order_timeout"`
		MakerFeeRate        float64 `json:"maker_fee_rate"`
		TakerFeeRate        float64 `json:"taker_fee_rate"`
		ExtraCostPct        float64 `json:"extra_cost_pct"`
		MaxStopLossPct      float64 `json:"max_stop_loss_pct"`
		MinNetProfitPct     float64 `json:"min_net_profit_pct"`
	} `json:"execution"`

	Concurrency int           `json:"concurrency"`
	HTTPTimeout time.Duration `json:"-"`
	RunTimeout  time.Duration `json:"-"`
	MaxRetries  int           `json:"max_retries"`
	RetryDelay  time.Duration `json:"-"`

	Output struct {
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
		KlineLimit5m      int `json:"kline_limit_5m"`
		KlineLimit15m     int `json:"kline_limit_15m"`
		KlineLimit30m     int `json:"kline_limit_30m"`
		KlineLimit1h      int `json:"kline_limit_1h"`
		KlineLimit4h      int `json:"kline_limit_4h"`
		OpenInterestLimit int `json:"open_interest_limit"`
		FundingLimit      int `json:"funding_limit"`
		OrderBookLimit    int `json:"order_book_limit"`
		MaxDataAgeSeconds int `json:"max_data_age_seconds"`
	} `json:"analysis"`

	Execution struct {
		Testnet             bool    `json:"testnet"`
		MaxLeverage         int     `json:"max_leverage"`
		MarginPerTradeUSD   float64 `json:"margin_per_trade_usd"`
		MaxTotalMarginUSD   float64 `json:"max_total_margin_usd"`
		MaxActivePositions  int     `json:"max_active_positions"`
		MinScore            float64 `json:"min_score"`
		TrailingPct         float64 `json:"trailing_pct"`
		CheckInterval       string  `json:"check_interval"`
		PendingOrderTimeout string  `json:"pending_order_timeout"`
		MakerFeeRate        float64 `json:"maker_fee_rate"`
		TakerFeeRate        float64 `json:"taker_fee_rate"`
		ExtraCostPct        float64 `json:"extra_cost_pct"`
		MaxStopLossPct      float64 `json:"max_stop_loss_pct"`
		MinNetProfitPct     float64 `json:"min_net_profit_pct"`
	} `json:"execution"`

	Concurrency int    `json:"concurrency"`
	HTTPTimeout string `json:"http_timeout"`
	RunTimeout  string `json:"run_timeout"`
	MaxRetries  int    `json:"max_retries"`
	RetryDelay  string `json:"retry_delay"`

	Output struct {
		File string `json:"file"`
	} `json:"output"`
}

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

	if cfg.Bybit.BaseURL == "" {
		cfg.Bybit.BaseURL = "https://api.bybit.com"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	if cfg.Filters.PreselectCandidates <= 0 {
		cfg.Filters.PreselectCandidates = 60
	}
	if cfg.Filters.TopCandidates <= 0 {
		cfg.Filters.TopCandidates = 20
	}
	if cfg.Analysis.KlineLimit5m <= 0 {
		cfg.Analysis.KlineLimit5m = 300
	}
	if cfg.Analysis.KlineLimit15m <= 0 {
		cfg.Analysis.KlineLimit15m = 300
	}
	if cfg.Analysis.KlineLimit30m <= 0 {
		cfg.Analysis.KlineLimit30m = 300
	}
	if cfg.Analysis.KlineLimit1h <= 0 {
		cfg.Analysis.KlineLimit1h = 300
	}
	if cfg.Analysis.KlineLimit4h <= 0 {
		cfg.Analysis.KlineLimit4h = 300
	}
	if cfg.Analysis.OpenInterestLimit <= 0 {
		cfg.Analysis.OpenInterestLimit = 24
	}
	if cfg.Analysis.FundingLimit <= 0 {
		cfg.Analysis.FundingLimit = 20
	}
	if cfg.Analysis.OrderBookLimit <= 0 {
		cfg.Analysis.OrderBookLimit = 50
	}
	if cfg.Analysis.MaxDataAgeSeconds <= 0 {
		cfg.Analysis.MaxDataAgeSeconds = 30
	}
	if cfg.Execution.MaxTotalMarginUSD <= 0 {
		cfg.Execution.MaxTotalMarginUSD = 20
	}
	if cfg.Execution.MaxActivePositions <= 0 {
		cfg.Execution.MaxActivePositions = 3
	}
	if cfg.Execution.PendingOrderTimeout == "" {
		cfg.Execution.PendingOrderTimeout = "5m"
	}
	if cfg.Execution.MakerFeeRate <= 0 {
		cfg.Execution.MakerFeeRate = 0.0002
	}
	if cfg.Execution.TakerFeeRate <= 0 {
		cfg.Execution.TakerFeeRate = 0.00055
	}
	if cfg.Execution.ExtraCostPct < 0 {
		cfg.Execution.ExtraCostPct = 0
	}
	if cfg.Execution.MaxStopLossPct <= 0 {
		cfg.Execution.MaxStopLossPct = 5
	}
	if cfg.Execution.MinNetProfitPct <= 0 {
		cfg.Execution.MinNetProfitPct = 0.5
	}

	return cfg, nil
}
