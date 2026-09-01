// models/market.go
package models

import "time"

// Instrument описывает USDT Linear Perpetual контракт на Bybit
type Instrument struct {
	Symbol          string    `json:"symbol"`
	BaseCoin        string    `json:"base_coin"`
	QuoteCoin       string    `json:"quote_coin"`
	SettleCoin      string    `json:"settle_coin"`
	ContractType    string    `json:"contract_type"`
	Status          string    `json:"status"`
	LaunchTime      time.Time `json:"launch_time"`
	TickSize        float64   `json:"tick_size"`
	QtyStep         float64   `json:"qty_step"`
	MinOrderQty     float64   `json:"min_order_qty"`
	MinNotional     float64   `json:"min_notional"`
	FundingInterval int       `json:"funding_interval_min"`
}

// Ticker содержит текущий 24h снимок цены и объема
type Ticker struct {
	Symbol            string  `json:"symbol"`
	LastPrice         float64 `json:"last_price"`
	MarkPrice         float64 `json:"mark_price"`
	IndexPrice        float64 `json:"index_price"`
	Price24hPcnt      float64 `json:"price_24h_pct"`
	HighPrice24h      float64 `json:"high_24h"`
	LowPrice24h       float64 `json:"low_24h"`
	Turnover24h       float64 `json:"turnover_24h"`
	Volume24h         float64 `json:"volume_24h"`
	Bid1Price         float64 `json:"bid1_price"`
	Ask1Price         float64 `json:"ask1_price"`
	FundingRate       float64 `json:"funding_rate"`
	OpenInterest      float64 `json:"open_interest"`
	OpenInterestValue float64 `json:"open_interest_value"`
}

// Candle представляет OHLCV свечу
type Candle struct {
	Time     time.Time `json:"time"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
	Turnover float64   `json:"turnover"`
}

// OpenInterestPoint — элемент исторического снимка OI
type OpenInterestPoint struct {
	Time         time.Time `json:"time"`
	OpenInterest float64   `json:"open_interest"`
}

// FundingPoint — элемент истории фандинга
type FundingPoint struct {
	Time time.Time `json:"time"`
	Rate float64   `json:"rate"`
}

// Indicators хранит вычисленные технические индикаторы
type Indicators struct {
	RSI15m        float64 `json:"rsi_15m"`
	RSI1h         float64 `json:"rsi_1h"`
	RSI4h         float64 `json:"rsi_4h"`
	ATR15m        float64 `json:"atr_15m"`
	ATR1h         float64 `json:"atr_1h"`
	ATR4h         float64 `json:"atr_4h"`
	ATR1hPct      float64 `json:"atr_1h_pct"`
	ATR4hPct      float64 `json:"atr_4h_pct"`
	VolumeRatio1h float64 `json:"volume_ratio_1h"`
	VolumeTrend1h float64 `json:"volume_trend_1h"`
}

// Pivot определяет экстремум структуры
type Pivot struct {
	Time  time.Time `json:"time"`
	Price float64   `json:"price"`
}

// Structure содержит структуру экстремумов таймфрейма
type Structure struct {
	Highs        []Pivot `json:"pivot_highs"`
	Lows         []Pivot `json:"pivot_lows"`
	HighState    string  `json:"high_state"` // HH, LH, EQ
	LowState     string  `json:"low_state"`  // HL, LL, EQ
	PreviousHigh float64 `json:"previous_high"`
	CurrentHigh  float64 `json:"current_high"`
	PreviousLow  float64 `json:"previous_low"`
	CurrentLow   float64 `json:"current_low"`
}

// Levels хранит ценовые уровни и геометрические параметры диапазона
type Levels struct {
	Resistance        []float64 `json:"resistance"`
	Support           []float64 `json:"support"`
	NearestResistance float64   `json:"nearest_resistance"`
	NearestSupport    float64   `json:"nearest_support"`
	RangeWidthPct     float64   `json:"range_width_pct"`
	RangePositionPct  float64   `json:"range_position_pct"`
	RangeToATR1h      float64   `json:"range_to_atr_1h"`
}

// Derivatives содержит деривативные свойства
type Derivatives struct {
	FundingRate        float64 `json:"funding_rate"`
	FundingAvg         float64 `json:"funding_avg"`
	FundingAvg24h      float64 `json:"funding_avg_24h"`
	OpenInterest       float64 `json:"open_interest"`
	OpenInterestChange float64 `json:"open_interest_change_pct"`
	SpreadPct          float64 `json:"spread_pct"`
}

// OrderBookMetrics содержит расчет мгновенного баланса стакана L2
type OrderBookMetrics struct {
	BidNotional  float64 `json:"bid_notional"`
	AskNotional  float64 `json:"ask_notional"`
	ImbalancePct float64 `json:"imbalance_pct"`
	BidAskRatio  float64 `json:"bid_ask_ratio"`
	Levels       int     `json:"levels"`
}

// StrategyResult хранит итоговую оценку алгоритма
type StrategyResult struct {
	Score  float64 `json:"score"`
	Status string  `json:"status"` // consider, watch, risky, avoid, reject
	Reason string  `json:"reason"`
}

// Candidate — агрегированный вектор данных монеты
type Candidate struct {
	Symbol string `json:"symbol"`
	Market struct {
		Price       float64 `json:"price"`
		Change24h   float64 `json:"change_24h_pct"`
		Change3d    float64 `json:"change_3d_pct"`
		Change7d    float64 `json:"change_7d_pct"`
		Turnover24h float64 `json:"turnover_24h"`
		Volume24h   float64 `json:"volume_24h"`
		SpreadPct   float64 `json:"spread_pct"`
	} `json:"market"`
	Indicators  Indicators                `json:"indicators"`
	Structure   map[string]Structure      `json:"structure"`
	Levels      Levels                    `json:"levels"`
	Derivatives Derivatives               `json:"derivatives"`
	OrderBook   OrderBookMetrics          `json:"order_book"`
	Strategies  map[string]StrategyResult `json:"strategies"`
}

// ScreeningResult — итоговый контракт данных на диск
type ScreeningResult struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Strategy    string      `json:"primary_strategy"`
	Prompt      string      `json:"prompt"`
	Filters     any         `json:"filters"`
	Candidates  []Candidate `json:"candidates"`
}
