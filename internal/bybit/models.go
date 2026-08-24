package bybit

// Position — наша внутренняя модель позиции.
//
// Важный момент:
//
// Мы сохраняем большинство числовых значений как string.
//
// Почему?
//
// Потому что Bybit API отдаёт цены, размеры и PnL как строки:
//
//	"0.03274"
//	"300"
//	"-0.033"
//
// Если сразу преобразовывать их в float64,
// можно получить ошибки округления.
//
// Например:
//
//	0.1 + 0.2 != 0.3
//
// для бинарного floating point.
//
// Для мониторинга это особенно важно,
// поэтому пока сохраняем оригинальные значения.
type Position struct {
	PositionIdx int `json:"positionIdx"`

	Symbol string `json:"symbol"`
	Side   string `json:"side"`

	Size     string `json:"size"`
	Leverage string `json:"leverage"`

	AvgPrice  string `json:"avgPrice"`
	MarkPrice string `json:"markPrice"`
	LiqPrice  string `json:"liqPrice"`

	PositionValue   string `json:"positionValue"`
	PositionBalance string `json:"positionBalance"`

	PositionIM string `json:"positionIM"`
	PositionMM string `json:"positionMM"`

	UnrealisedPnl  string `json:"unrealisedPnl"`
	CumRealisedPnl string `json:"cumRealisedPnl"`
	CurRealisedPnl string `json:"curRealisedPnl"`

	TakeProfit   string `json:"takeProfit"`
	StopLoss     string `json:"stopLoss"`
	TrailingStop string `json:"trailingStop"`

	BreakEvenPrice string `json:"breakEvenPrice"`

	OpenTime    FlexibleInt64 `json:"openTime"`
	UpdatedTime FlexibleInt64 `json:"updatedTime"`

	PositionStatus string `json:"positionStatus"`

	IsReduceOnly bool `json:"isReduceOnly"`
}

// PositionResponse — полный ответ REST API.
//
// Нам сейчас нужны только result.list,
// но структура отражает реальный формат Bybit.
type PositionResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`

	Result struct {
		NextPageCursor string     `json:"nextPageCursor"`
		Category       string     `json:"category"`
		List           []Position `json:"list"`
	} `json:"result"`
}
