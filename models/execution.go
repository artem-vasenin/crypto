package models

import "time"

type BotConfig struct {
	ApiKey              string        `json:"-"`
	ApiSecret           string        `json:"-"`
	Testnet             bool          `json:"testnet"`
	MaxLeverage         int           `json:"max_leverage"`
	MarginPerTradeUSD   float64       `json:"margin_per_trade_usd"`
	MaxTotalMarginUSD   float64       `json:"max_total_margin_usd"`
	MaxActivePositions  int           `json:"max_active_positions"`
	MinScore            float64       `json:"min_score"`
	TrailingPct         float64       `json:"trailing_pct"`
	CheckInterval       time.Duration `json:"-"`
	PendingOrderTimeout time.Duration `json:"-"`
	MakerFeeRate        float64       `json:"maker_fee_rate"`
	TakerFeeRate        float64       `json:"taker_fee_rate"`
	ExtraCostPct        float64       `json:"extra_cost_pct"`
	MaxStopLossPct      float64       `json:"max_stop_loss_pct"`
	MinNetProfitPct     float64       `json:"min_net_profit_pct"`
}

type PositionState struct {
	Symbol               string    `json:"symbol"`
	Side                 string    `json:"side"`
	OrderID              string    `json:"order_id"`
	EntryPrice           float64   `json:"entry_price"`
	Size                 float64   `json:"size"`
	MarginUSD            float64   `json:"margin_usd"`
	Leverage             int       `json:"leverage"`
	StopLoss             float64   `json:"stop_loss"`
	TakeProfit           float64   `json:"take_profit"`
	HighestPrice         float64   `json:"highest_price"`
	LowestPrice          float64   `json:"lowest_price"`
	OpenedAt             time.Time `json:"opened_at"`
	RiskAttached         bool      `json:"risk_attached"`
	SnapshotSaved        bool      `json:"snapshot_saved"`
	RiskAttachInProgress bool      `json:"risk_attach_in_progress"`
	Managed              bool      `json:"managed"`
	Pending              bool      `json:"pending"`
}
