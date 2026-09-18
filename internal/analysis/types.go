// Package analysis описывает evidence-first JSON-отчёт универсального анализатора.
package analysis

import (
	"crypto-coin-analyzer/internal/bybit"
	"time"
)

// Request хранит выбранный пользователем режим. Raw evidence от выбора не зависит и сохраняется всегда.
type Request struct {
	AnalysisType string `json:"analysis_type"`
	Direction    string `json:"direction"`
}

// Report — самодостаточный отчёт по одному символу. В версии 3.1 намеренно нет интегральных score:
// факторы разной природы нельзя корректно свести простым сложением в одно число без проверенной модели весов.
type Report struct {
	SchemaVersion   string                       `json:"schema_version"`
	AnalyzerVersion string                       `json:"analyzer_version"`
	GeneratedAt     time.Time                    `json:"generated_at"`
	Exchange        string                       `json:"exchange"`
	Category        string                       `json:"category"`
	Symbol          string                       `json:"symbol"`
	Request         Request                      `json:"request"`
	Purpose         string                       `json:"purpose"`
	DataQuality     DataQuality                  `json:"data_quality"`
	Market          Market                       `json:"market"`
	Timeframes      map[string]Timeframe         `json:"timeframes"`
	Structures      map[string]StructureAnalysis `json:"structures"`
	MarketRegime    MarketRegime                 `json:"market_regime"`
	RegimeHistory   []RegimeSnapshot             `json:"regime_history"`
	Range           RangeAnalysis                `json:"range_analysis"`
	Grid            GridAnalysis                 `json:"grid_analysis"`
	Directional     DirectionalAnalysis          `json:"directional_analysis"`
	Derivatives     Derivatives                  `json:"derivatives"`
	Microstructure  Microstructure               `json:"microstructure"`
	BTCContext      BTCContext                   `json:"btc_context"`
	RawEvidence     RawEvidence                  `json:"raw_evidence"`
	AIInstructions  AIInstructions               `json:"ai_instructions"`
}
type DataQuality struct {
	Complete        bool              `json:"complete"`
	Warnings        []string          `json:"warnings,omitempty"`
	Counts          map[string]int    `json:"counts"`
	RequestedDepth  map[string]string `json:"requested_depth"`
	EvidenceQuality map[string]string `json:"evidence_quality"`
}
type Market struct {
	Price             float64 `json:"price"`
	Change24hPct      float64 `json:"change_24h_pct"`
	High24h           float64 `json:"high_24h"`
	Low24h            float64 `json:"low_24h"`
	Turnover24h       float64 `json:"turnover_24h"`
	Volume24h         float64 `json:"volume_24h"`
	SpreadPct         float64 `json:"spread_pct"`
	FundingRate       float64 `json:"funding_rate"`
	OpenInterest      float64 `json:"open_interest"`
	OpenInterestValue float64 `json:"open_interest_value"`
}
type Timeframe struct {
	Candles           int     `json:"candles"`
	ChangePct         float64 `json:"change_pct"`
	RSI14             float64 `json:"rsi_14"`
	ATR14             float64 `json:"atr_14"`
	ATRPct            float64 `json:"atr_pct"`
	ADX14             float64 `json:"adx_14"`
	EMA20             float64 `json:"ema_20"`
	EMA50             float64 `json:"ema_50"`
	EMA200            float64 `json:"ema_200"`
	EfficiencyRatio   float64 `json:"efficiency_ratio"`
	RealizedVolPct    float64 `json:"realized_vol_pct"`
	BollingerWidthPct float64 `json:"bollinger_width_pct"`
	VolumeRatio       float64 `json:"volume_ratio"`
	High              float64 `json:"window_high"`
	Low               float64 `json:"window_low"`
	RangePositionPct  float64 `json:"range_position_pct"`
}

// SwingPoint сохраняет координаты подтверждённого локального экстремума, чтобы ИИ мог проверить label структуры.
type SwingPoint struct {
	Time     time.Time `json:"time"`
	Type     string    `json:"type"`
	Price    float64   `json:"price"`
	Strength int       `json:"strength_bars"`
	// Ambiguous=true означает outside-bar, который одновременно является pivot high и pivot low.
	// Такая точка сохраняется как raw/derived evidence, но не участвует в HH/HL/LH/LL последовательности.
	Ambiguous bool `json:"ambiguous_outside_bar"`
}
type StructureAnalysis struct {
	Label             string       `json:"label"`
	SwingHighSequence string       `json:"swing_high_sequence"`
	SwingLowSequence  string       `json:"swing_low_sequence"`
	LastSwings        []SwingPoint `json:"last_swings"`
	Evidence          []string     `json:"evidence"`
	Conflicts         []string     `json:"conflicts"`
}
type MarketRegime struct {
	Classification  string   `json:"classification"`
	TrendStrength   string   `json:"trend_strength"`
	VolatilityState string   `json:"volatility_state"`
	BreakoutRisk    string   `json:"breakout_risk"`
	DirectionalBias string   `json:"directional_bias"`
	Evidence        []string `json:"evidence"`
	Conflicts       []string `json:"conflicts"`
}
type RegimeSnapshot struct {
	Time                 time.Time `json:"time"`
	LookbackHours        int       `json:"lookback_hours"`
	Classification       string    `json:"classification"`
	ADX                  float64   `json:"adx"`
	EfficiencyRatio      float64   `json:"efficiency_ratio"`
	ATRPct               float64   `json:"atr_pct"`
	VolumeRatio          float64   `json:"volume_ratio"`
	RangeDriftPctPerHour float64   `json:"range_drift_pct_per_hour"`
}

// RangeAnalysis отделяет геометрию range от его стационарности. Moving channel не должен выглядеть как хороший боковик.
type RangeAnalysis struct {
	LookbackHours         int      `json:"lookback_hours"`
	High                  float64  `json:"high"`
	Low                   float64  `json:"low"`
	Mid                   float64  `json:"mid"`
	WidthPct              float64  `json:"width_pct"`
	PositionPct           float64  `json:"position_pct"`
	UpperTouches          int      `json:"upper_touches"`
	LowerTouches          int      `json:"lower_touches"`
	MidCrosses            int      `json:"mid_crosses"`
	FalseBreaksUp         int      `json:"false_breaks_up"`
	FalseBreaksDown       int      `json:"false_breaks_down"`
	CloseOutsidePct       float64  `json:"close_outside_pct"`
	SlopePctPerHour       float64  `json:"mid_slope_pct_per_hour"`
	RollingMidDriftPct    float64  `json:"rolling_mid_drift_pct"`
	RollingWidthChangePct float64  `json:"rolling_width_change_pct"`
	Stationarity          string   `json:"stationarity"`
	BoundaryBalance       string   `json:"boundary_balance"`
	Evidence              []string `json:"evidence"`
	Risks                 []string `json:"risks"`
}

// SideAssessment — не рейтинг. Здесь только независимые признаки и блокирующие/предупреждающие условия.
type SideAssessment struct {
	State             string   `json:"state"`
	PrimaryEvidence   []string `json:"primary_evidence"`
	SecondaryEvidence []string `json:"secondary_evidence"`
	RiskFactors       []string `json:"risk_factors"`
	HardBlocks        []string `json:"hard_blocks"`
	EntryContext      string   `json:"entry_context"`
}
type GridAnalysis struct {
	Regime             string         `json:"regime"`
	RangeState         string         `json:"range_state"`
	MeanReversionState string         `json:"mean_reversion_state"`
	BreakoutRisk       string         `json:"breakout_risk"`
	Long               SideAssessment `json:"long"`
	Short              SideAssessment `json:"short"`
	Evidence           []string       `json:"evidence"`
	RiskFactors        []string       `json:"risk_factors"`
}
type DirectionalAnalysis struct {
	Long                SideAssessment `json:"long"`
	Short               SideAssessment `json:"short"`
	BullishEvidence     []string       `json:"bullish_evidence"`
	BearishEvidence     []string       `json:"bearish_evidence"`
	ConflictingEvidence []string       `json:"conflicting_evidence"`
	InvalidationContext []string       `json:"invalidation_context"`
	TargetContext       []string       `json:"target_context"`
}

type Derivatives struct {
	FundingHistory              []bybit.Funding      `json:"funding_history"`
	FundingCurrent              float64              `json:"funding_current"`
	FundingAvg24h               float64              `json:"funding_avg_24h"`
	FundingAvg7d                float64              `json:"funding_avg_7d"`
	FundingPercentile           float64              `json:"funding_percentile"`
	OI5m                        []bybit.OpenInterest `json:"open_interest_5m"`
	OIChange15mPct              float64              `json:"oi_change_15m_pct"`
	OIChange1hPct               float64              `json:"oi_change_1h_pct"`
	OIChange4hPct               float64              `json:"oi_change_4h_pct"`
	OIChange24hPct              float64              `json:"oi_change_24h_pct"`
	PriceOIState1h              string               `json:"price_oi_state_1h"`
	PriceOIState4h              string               `json:"price_oi_state_4h"`
	LongShort5m                 []bybit.LongShort    `json:"long_short_5m"`
	LongRatioNow                float64              `json:"long_ratio_now"`
	LongRatioChange1hPctPoints  float64              `json:"long_ratio_change_1h_pct_points"`
	LongRatioChange4hPctPoints  float64              `json:"long_ratio_change_4h_pct_points"`
	LongRatioChange24hPctPoints float64              `json:"long_ratio_change_24h_pct_points"`
}
type FlowWindow struct {
	Window                  string  `json:"window"`
	Available               bool    `json:"available"`
	Status                  string  `json:"status"`
	RequiredCoverageSeconds float64 `json:"required_coverage_seconds"`
	ActualCoverageSeconds   float64 `json:"actual_coverage_seconds"`
	Trades                  int     `json:"trades"`
	BuyNotional             float64 `json:"buy_notional"`
	SellNotional            float64 `json:"sell_notional"`
	DeltaPct                float64 `json:"delta_pct"`
}
type Microstructure struct {
	OrderBook             bybit.OrderBook `json:"order_book"`
	RecentTradesCount     int             `json:"recent_trades_count"`
	TradesFirstTime       time.Time       `json:"trades_first_time"`
	TradesLastTime        time.Time       `json:"trades_last_time"`
	TradesCoverageSeconds float64         `json:"trades_coverage_seconds"`
	TakerBuyNotional      float64         `json:"taker_buy_notional"`
	TakerSellNotional     float64         `json:"taker_sell_notional"`
	TakerDeltaPct         float64         `json:"taker_delta_pct"`
	TakerFlowWindows      []FlowWindow    `json:"taker_flow_windows"`
	MarkPrice             float64         `json:"mark_price"`
	IndexPrice            float64         `json:"index_price"`
	MarkVsIndexPct        float64         `json:"mark_vs_index_pct"`
	LastVsMarkPct         float64         `json:"last_vs_mark_pct"`
}
type BTCContext struct {
	Price                  float64 `json:"price"`
	Change1hPct            float64 `json:"change_1h_pct"`
	Change4hPct            float64 `json:"change_4h_pct"`
	Change24hPct           float64 `json:"change_24h_pct"`
	Correlation1h30d       float64 `json:"correlation_1h_30d"`
	RelativeStrength24hPct float64 `json:"relative_strength_24h_pct"`
}
type RawEvidence struct {
	Candles5m         []bybit.Candle      `json:"candles_5m"`
	Candles15m        []bybit.Candle      `json:"candles_15m"`
	Candles1h         []bybit.Candle      `json:"candles_1h"`
	Candles4h         []bybit.Candle      `json:"candles_4h"`
	Candles1d         []bybit.Candle      `json:"candles_1d"`
	RecentTrades      []bybit.Trade       `json:"recent_trades"`
	MarkPriceHistory  []bybit.PriceCandle `json:"mark_price_history"`
	IndexPriceHistory []bybit.PriceCandle `json:"index_price_history"`
}
type AIInstructions struct {
	Task                   string   `json:"task"`
	AnalysisOrder          []string `json:"analysis_order"`
	ImportantCautions      []string `json:"important_cautions"`
	ExpectedDecisionFields []string `json:"expected_decision_fields"`
}
