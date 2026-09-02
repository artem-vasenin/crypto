// models/execution.go
package models

import "time"

type BotConfig struct {
	ApiKey             string        `json:"-"`
	ApiSecret          string        `json:"-"`
	Testnet            bool          `json:"testnet"`
	MaxLeverage        int           `json:"max_leverage"`
	MarginPerTradeUSD  float64       `json:"margin_per_trade_usd"`
	MaxTotalMarginUSD  float64       `json:"max_total_margin_usd"`
	MaxActivePositions int           `json:"max_active_positions"`
	MinScore           float64       `json:"min_score"`
	TrailingPct        float64       `json:"trailing_pct"`
	CheckInterval      time.Duration `json:"-"`
}

type PositionState struct {
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	EntryPrice   float64   `json:"entry_price"`
	Size         float64   `json:"size"`
	StopLoss     float64   `json:"stop_loss"`
	HighestPrice float64   `json:"highest_price"`
	LowestPrice  float64   `json:"lowest_price"`
	OpenedAt     time.Time `json:"opened_at"`
}
