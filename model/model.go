package model

// Candle представляет одну свечу Bybit.
//
// Мы намеренно не храним здесь все поля ответа API.
// Для анализа нам нужны только основные значения.
type Candle struct {
	Time   int64
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// Ticker содержит текущую рыночную информацию по инструменту.
type Ticker struct {
	Symbol string

	// Последняя цена.
	LastPrice float64

	// Изменение цены за 24 часа в процентах.
	Change24h float64

	// Оборот за последние 24 часа в USDT.
	Turnover24h float64

	// Funding rate.
	//
	// Например:
	// 0.0001 = 0.01%
	FundingRate float64

	// Open Interest.
	OpenInterest      float64
	OpenInterestValue float64

	// Лучшая цена покупки/продажи.
	BidPrice float64
	AskPrice float64

	// Размеры лучших заявок.
	BidSize float64
	AskSize float64
}

// Instrument содержит информацию о торговом контракте.
type Instrument struct {
	Symbol       string
	BaseCoin     string
	QuoteCoin    string
	ContractType string
	Status       string

	// Время запуска контракта.
	LaunchTime int64

	// Для perpetual обычно 0.
	DeliveryTime int64

	// Интервал funding в часах.
	FundingIntervalHour int
}

// Structure описывает структуру цены на конкретном таймфрейме.
type Structure struct {
	Interval string

	High float64
	Low  float64

	// Диапазон между High и Low относительно Low.
	RangePercent float64

	// Насколько текущая цена ниже максимума.
	FromHighPercent float64

	// Положение цены внутри диапазона:
	//
	// 0   = возле Low
	// 100 = возле High
	PositionPercent float64

	// Последние pivot highs.
	PivotHighs []Pivot

	// Последние pivot lows.
	PivotLows []Pivot

	// Структура максимумов:
	// HH / LH / NONE.
	HighStructure string

	// Изменение между последними двумя pivot highs.
	HighStructurePercent float64

	// Предыдущий максимум.
	PreviousHigh float64

	// Текущий максимум.
	CurrentHigh float64

	// Структура минимумов:
	// HL / LL / NONE.
	LowStructure string

	// Изменение между последними двумя pivot lows.
	LowStructurePercent float64

	PreviousLow float64
	CurrentLow  float64
}

// Pivot — локальный максимум или минимум.
type Pivot struct {
	Time  int64
	Price float64
}

// IndicatorData содержит технические индикаторы.
type IndicatorData struct {
	RSI15m float64
	RSI1h  float64
	RSI4h  float64

	ATR15m float64
	ATR1h  float64
	ATR4h  float64

	ATR15mPercent float64
	ATR1hPercent  float64
	ATR4hPercent  float64

	// Отношение объёма последней закрытой свечи
	// к среднему объёму предыдущих свечей.
	VolumeRatio15m float64
	VolumeRatio1h  float64
	VolumeRatio4h  float64
}

// OIAnalysis содержит анализ Open Interest.
type OIAnalysis struct {
	Current float64

	// Изменение OI за выбранное окно.
	ChangePercent float64
}

// Candidate — полностью проанализированный кандидат.
type Candidate struct {
	Symbol string

	Price       float64
	Change24h   float64
	Change7d    float64
	Turnover24h float64

	FundingRate float64

	OpenInterest      float64
	OpenInterestValue float64

	// Bid/Ask spread.
	SpreadPercent float64

	Indicators IndicatorData

	Structure4h  Structure
	Structure1h  Structure
	Structure15m Structure

	OI OIAnalysis

	// Ближайшее сопротивление.
	NearestResistance float64

	// Расстояние от цены до сопротивления.
	ResistanceDistancePercent float64

	// Пока это не финальный trading score.
	//
	// Здесь будет наша следующая стадия.
	Score float64
}

// Report — весь результат одного запуска скринера.
type Report struct {
	GeneratedAt string

	Filters struct {
		MaxPrice       float64
		MinTurnover24h float64
	}

	TotalInstruments int
	TotalCandidates  int

	Candidates []Candidate
}
