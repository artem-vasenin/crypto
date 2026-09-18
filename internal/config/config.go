package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Thresholds содержит калибруемые границы. Они не образуют интегральный score.
type Thresholds struct {
	DriftATRWeak         float64 `json:"drift_atr_weak"`
	DriftATRStrong       float64 `json:"drift_atr_strong"`
	ADXTrend             float64 `json:"adx_trend"`
	ADXStrong            float64 `json:"adx_strong"`
	EfficiencyTrend      float64 `json:"efficiency_trend"`
	ATRExpansionRatio    float64 `json:"atr_expansion_ratio"`
	VolumeExpansionRatio float64 `json:"volume_expansion_ratio"`
	MTFConflictBlock     bool    `json:"mtf_conflict_block"`
}

// Config содержит эксплуатационные настройки приложения.
type Config struct {
	BybitBaseURL             string     `json:"bybit_base_url"`
	OutputDir                string     `json:"output_dir"`
	MinTurnover24hUSDT       float64    `json:"min_turnover_24h_usdt"`
	MinHistoryBars           int        `json:"min_history_bars"`
	MaxConcurrency           int        `json:"max_concurrency"`
	RequestTimeoutSeconds    int        `json:"request_timeout_seconds"`
	GridReferenceCapitalUSDT float64    `json:"grid_reference_capital_usdt"`
	GridMinLevels            int        `json:"grid_min_levels"`
	GridMaxPriceUSDT         float64    `json:"grid_max_price_usdt"`
	Thresholds               Thresholds `json:"thresholds"`
}

// Load читает конфигурацию JSON и проверяет критические поля.
func Load(path string) (Config, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return Config{}, e
	}
	var c Config
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if c.BybitBaseURL == "" {
		c.BybitBaseURL = "https://api.bybit.com"
	}
	if c.OutputDir == "" {
		c.OutputDir = "output"
	}
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = 8
	}
	if c.RequestTimeoutSeconds <= 0 {
		c.RequestTimeoutSeconds = 15
	}
	if c.MinHistoryBars < 100 {
		return c, fmt.Errorf("min_history_bars должен быть >= 100")
	}
	return c, nil
}
