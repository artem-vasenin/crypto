package domain

import "time"

// Candle представляет нормализованную свечу OHLCV.
type Candle struct {
	OpenTime                                 time.Time `json:"open_time"`
	Open, High, Low, Close, Volume, Turnover float64
}

// Instrument содержит торговые ограничения инструмента Bybit.
type Instrument struct {
	Symbol                                      string
	Status                                      string
	TickSize, QtyStep, MinOrderQty, MinNotional float64
}

// Ticker содержит массовые рыночные данные тикера.
type Ticker struct {
	Symbol                                                       string
	LastPrice, Turnover24h, Volume24h, FundingRate, OpenInterest float64
}

// Direction — объективное направление рынка, не торговая команда.
type Direction string

const (
	DirectionUp         Direction = "UP"
	DirectionDown       Direction = "DOWN"
	DirectionSideways   Direction = "SIDEWAYS"
	DirectionTransition Direction = "TRANSITION"
	DirectionUnknown    Direction = "UNKNOWN"
)

// Strength описывает силу установленного направления.
type Strength string

const (
	StrengthWeak     Strength = "WEAK"
	StrengthModerate Strength = "MODERATE"
	StrengthStrong   Strength = "STRONG"
	StrengthUnknown  Strength = "UNKNOWN"
)

// Dynamics описывает изменение силы движения.
type Dynamics string

const (
	DynamicsAccelerating Dynamics = "ACCELERATING"
	DynamicsStable       Dynamics = "STABLE"
	DynamicsDecelerating Dynamics = "DECELERATING"
	DynamicsUnknown      Dynamics = "UNKNOWN"
)

// StructureState описывает последовательность значимых экстремумов таймфрейма.
type StructureState string

const (
	StructureBull    StructureState = "HH_HL"
	StructureBear    StructureState = "LH_LL"
	StructureMixed   StructureState = "MIXED"
	StructureUnknown StructureState = "UNKNOWN"
)

// TimeframeFeatures хранит независимые признаки одного таймфрейма.
type TimeframeFeatures struct {
	Timeframe                                                                                                                  string         `json:"timeframe"`
	Bars                                                                                                                       int            `json:"bars"`
	LastPrice, ATR, ATRPct, ADX, PlusDI, MinusDI, Efficiency, Center, CenterDriftPct, CenterDriftATR, MomentumPct, VolumeRatio float64        `json:",omitempty"`
	Structure                                                                                                                  StructureState `json:"structure"`
}

// MarketSnapshot — единый результат общего ядра до интерпретации стратегией.
type MarketSnapshot struct {
	Symbol                                        string                       `json:"symbol"`
	GeneratedAt                                   time.Time                    `json:"generated_at"`
	Price, Turnover24h, FundingRate, OpenInterest float64                      `json:",omitempty"`
	TF                                            map[string]TimeframeFeatures `json:"timeframes"`
	Direction                                     Direction                    `json:"direction"`
	Strength                                      Strength                     `json:"strength"`
	Dynamics                                      Dynamics                     `json:"dynamics"`
	MTFAlignment                                  string                       `json:"mtf_alignment"`
	DataQuality                                   string                       `json:"data_quality"`
	Evidence                                      []string                     `json:"evidence"`
	Conflicts                                     []string                     `json:"conflicts,omitempty"`
}

// Candidate — объяснимый результат допуска к более глубокому анализу.
type Candidate struct {
	Symbol          string             `json:"symbol"`
	Decision        string             `json:"decision"`
	Direction       Direction          `json:"direction"`
	Strength        Strength           `json:"strength"`
	Dynamics        Dynamics           `json:"dynamics"`
	MTFAlignment    string             `json:"mtf_alignment"`
	PrimaryEvidence []string           `json:"primary_evidence"`
	Confirmations   []string           `json:"confirmations,omitempty"`
	RiskFlags       []string           `json:"risk_flags,omitempty"`
	Metrics         map[string]float64 `json:"metrics,omitempty"`
}

// ScreeningFile — стабильный формат конечного JSON-файла.
type ScreeningFile struct {
	SchemaVersion   string      `json:"schema_version"`
	ScreenerVersion string      `json:"screener_version"`
	GeneratedAt     time.Time   `json:"generated_at"`
	Mode            string      `json:"mode"`
	Direction       string      `json:"direction"`
	Candidates      []Candidate `json:"candidates"`
}
