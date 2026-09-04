package analysis

import "time"

type Report struct {
	GeneratedAt    time.Time      `json:"generated_at"`
	Exchange       string         `json:"exchange"`
	Category       string         `json:"category"`
	Symbol         string         `json:"symbol"`
	Purpose        string         `json:"purpose"`
	DataQuality    DataQuality    `json:"data_quality"`
	Market         Market         `json:"market"`
	Indicators     Indicators     `json:"indicators"`
	Trend          Trend          `json:"trend"`
	Momentum       Momentum       `json:"momentum"`
	Volume         Volume         `json:"volume"`
	Structure      Structure      `json:"structure"`
	Levels         Levels         `json:"levels"`
	Derivatives    Derivatives    `json:"derivatives"`
	OrderBook      OrderBook      `json:"order_book"`
	BTCContext     BTCContext     `json:"btc_context"`
	Strategies     Strategies     `json:"strategies"`
	AIInstructions AIInstructions `json:"ai_instructions"`
}

type DataQuality struct {
	OneMinuteCandles int      `json:"one_minute_candles"`
	Notes            []string `json:"notes,omitempty"`
}
type Market struct {
	Price        float64 `json:"price"`
	Change24hPct float64 `json:"change_24h_pct"`
	Change3dPct  float64 `json:"change_3d_pct"`
	Change7dPct  float64 `json:"change_7d_pct"`
	Turnover24h  float64 `json:"turnover_24h"`
	Volume24h    float64 `json:"volume_24h"`
	SpreadPct    float64 `json:"spread_pct"`
}
type Indicators struct {
	RSI15m         float64 `json:"rsi_15m"`
	RSI1h          float64 `json:"rsi_1h"`
	RSI4h          float64 `json:"rsi_4h"`
	RSI4h14d       float64 `json:"rsi_4h_14d"` // RSI за 14 дней на 4h
	RSI4h30d       float64 `json:"rsi_4h_30d"` // RSI за 30 дней на 4h
	ATR15m         float64 `json:"atr_15m"`
	ATR1h          float64 `json:"atr_1h"`
	ATR4h          float64 `json:"atr_4h"`
	ATR4h14d       float64 `json:"atr_4h_14d"` // ATR за 14 дней
	ATR4h30d       float64 `json:"atr_4h_30d"` // ATR за 30 дней
	ATR1hPct       float64 `json:"atr_1h_pct"`
	ATR4hPct       float64 `json:"atr_4h_pct"`
	ATR4h14dPct    float64 `json:"atr_4h_14d_pct"` // ATR% за 14 дней
	ATR4h30dPct    float64 `json:"atr_4h_30d_pct"` // ATR% за 30 дней
	VolumeRatio1h  float64 `json:"volume_ratio_1h"`
	VolumeRatio14d float64 `json:"volume_ratio_14d"` // объём 14д vs 7д
}
type Trend struct {
	EMA20_15m              float64 `json:"ema20_15m"`
	EMA50_15m              float64 `json:"ema50_15m"`
	EMA200_15m             float64 `json:"ema200_15m"`
	EMA20_1h               float64 `json:"ema20_1h"`
	EMA50_1h               float64 `json:"ema50_1h"`
	EMA200_1h              float64 `json:"ema200_1h"`
	EMA20_4h               float64 `json:"ema20_4h"`
	EMA50_4h               float64 `json:"ema50_4h"`
	EMA200_4h              float64 `json:"ema200_4h"`
	EMA20_4h_14d           float64 `json:"ema20_4h_14d"`  // EMA20 за 14 дней
	EMA50_4h_14d           float64 `json:"ema50_4h_14d"`  // EMA50 за 14 дней
	EMA200_4h_14d          float64 `json:"ema200_4h_14d"` // EMA200 за 14 дней
	EMA20_4h_30d           float64 `json:"ema20_4h_30d"`  // EMA20 за 30 дней
	EMA50_4h_30d           float64 `json:"ema50_4h_30d"`  // EMA50 за 30 дней
	EMA200_4h_30d          float64 `json:"ema200_4h_30d"` // EMA200 за 30 дней
	PriceVsEMA20_1hPct     float64 `json:"price_vs_ema20_1h_pct"`
	PriceVsEMA50_1hPct     float64 `json:"price_vs_ema50_1h_pct"`
	PriceVsEMA200_1hPct    float64 `json:"price_vs_ema200_1h_pct"`
	PriceVsEMA20_4h_14dPct float64 `json:"price_vs_ema20_4h_14d_pct"`
	PriceVsEMA50_4h_14dPct float64 `json:"price_vs_ema50_4h_14d_pct"`
}
type Momentum struct {
	Change1hPct  float64 `json:"change_1h_pct"`
	Change4hPct  float64 `json:"change_4h_pct"`
	Change12hPct float64 `json:"change_12h_pct"`
	Change24hPct float64 `json:"change_24h_pct"`
	Change7dPct  float64 `json:"change_7d_pct"`  // уже есть в market, но дублируем для удобства
	Change14dPct float64 `json:"change_14d_pct"` // изменение за 14 дней
	Change30dPct float64 `json:"change_30d_pct"` // изменение за 30 дней
	ROC1hPct     float64 `json:"roc_1h_pct"`
	ROC4hPct     float64 `json:"roc_4h_pct"`
	ROC14dPct    float64 `json:"roc_14d_pct"` // ROC за 14 дней
}
type Volume struct {
	Volume5m  float64 `json:"volume_5m"`
	Volume15m float64 `json:"volume_15m"`
	Volume1h  float64 `json:"volume_1h"`
	Ratio5m   float64 `json:"ratio_5m"`
	Ratio15m  float64 `json:"ratio_15m"`
	Ratio1h   float64 `json:"ratio_1h"`
}
type Pivot struct {
	Time  time.Time `json:"time"`
	Price float64   `json:"price"`
}
type Structure struct {
	PivotHighs   []Pivot `json:"pivot_highs"`
	PivotLows    []Pivot `json:"pivot_lows"`
	HighState    string  `json:"high_state"`
	LowState     string  `json:"low_state"`
	PreviousHigh float64 `json:"previous_high"`
	CurrentHigh  float64 `json:"current_high"`
	PreviousLow  float64 `json:"previous_low"`
	CurrentLow   float64 `json:"current_low"`
}
type Levels struct {
	Resistance        []float64 `json:"resistance"`
	Support           []float64 `json:"support"`
	NearestResistance float64   `json:"nearest_resistance"`
	NearestSupport    float64   `json:"nearest_support"`
	RangeWidthPct     float64   `json:"range_width_pct"`
	RangePositionPct  float64   `json:"range_position_pct"`
	RangeToATR1h      float64   `json:"range_to_atr_1h"`
	PriceDiscovery    bool      `json:"price_discovery"`
	RecentRangeHigh   float64   `json:"recent_range_high"`
	RecentRangeLow    float64   `json:"recent_range_low"`
	// 14-дневные уровни
	Range14dHigh        float64 `json:"range_14d_high"`
	Range14dLow         float64 `json:"range_14d_low"`
	Range14dWidthPct    float64 `json:"range_14d_width_pct"`
	Range14dPositionPct float64 `json:"range_14d_position_pct"`
	Range14dToATR4h     float64 `json:"range_14d_to_atr_4h"`
	// 30-дневные уровни
	Range30dHigh        float64 `json:"range_30d_high"`
	Range30dLow         float64 `json:"range_30d_low"`
	Range30dWidthPct    float64 `json:"range_30d_width_pct"`
	Range30dPositionPct float64 `json:"range_30d_position_pct"`
}
type Derivatives struct {
	FundingRate           float64 `json:"funding_rate"`
	FundingAvg            float64 `json:"funding_avg"`
	FundingAvg24h         float64 `json:"funding_avg_24h"`
	OpenInterest          float64 `json:"open_interest"`
	OpenInterestChangePct float64 `json:"open_interest_change_pct"`
	LongRatio             float64 `json:"long_ratio"`
	ShortRatio            float64 `json:"short_ratio"`
}
type OrderBook struct {
	BidNotional  float64 `json:"bid_notional"`
	AskNotional  float64 `json:"ask_notional"`
	ImbalancePct float64 `json:"imbalance_pct"`
	BidAskRatio  float64 `json:"bid_ask_ratio"`
	SpreadPct    float64 `json:"spread_pct"`
	Levels       int     `json:"levels"`
}
type BTCContext struct {
	Price              float64 `json:"price"`
	Change1hPct        float64 `json:"change_1h_pct"`
	Change4hPct        float64 `json:"change_4h_pct"`
	Change24hPct       float64 `json:"change_24h_pct"`
	Corr1m0            float64 `json:"corr_1m_0"`
	Corr1m1            float64 `json:"corr_1m_1"`
	Corr1m2            float64 `json:"corr_1m_2"`
	Corr1m3            float64 `json:"corr_1m_3"`
	Corr1m5            float64 `json:"corr_1m_5"`
	Corr1m10           float64 `json:"corr_1m_10"`
	BestLagMinutes     int     `json:"best_lag_minutes"`
	BestLagCorrelation float64 `json:"best_lag_correlation"`
	Relative1hPct      float64 `json:"relative_1h_pct"`
	Relative4hPct      float64 `json:"relative_4h_pct"`
	Interpretation     string  `json:"interpretation"`
	SelfReference      bool    `json:"self_reference"`
}
type Strategy struct {
	Score  int    `json:"score"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type Strategies struct {
	Long        Strategy `json:"long"`
	LongGrid    Strategy `json:"long-grid"`
	NeutralGrid Strategy `json:"neutral-grid"`
	Short       Strategy `json:"short"`
	ShortGrid   Strategy `json:"short-grid"`
}
type AIInstructions struct {
	Task            string   `json:"task"`
	Rules           []string `json:"rules"`
	RequestedOutput []string `json:"requested_output"`
}
