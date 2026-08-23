package bybit

type Position struct {
	Symbol          string        `json:"symbol"`
	Side            string        `json:"side"`
	Size            string        `json:"size"`
	Leverage        string        `json:"leverage"`
	AvgPrice        string        `json:"avgPrice"`
	MarkPrice       string        `json:"markPrice"`
	LiqPrice        string        `json:"liqPrice"`
	PositionValue   string        `json:"positionValue"`
	PositionBalance string        `json:"positionBalance"`
	PositionIM      string        `json:"positionIM"`
	PositionMM      string        `json:"positionMM"`
	UnrealisedPnl   string        `json:"unrealisedPnl"`
	CumRealisedPnl  string        `json:"cumRealisedPnl"`
	CurRealisedPnl  string        `json:"curRealisedPnl"`
	TakeProfit      string        `json:"takeProfit"`
	StopLoss        string        `json:"stopLoss"`
	TrailingStop    string        `json:"trailingStop"`
	BreakEvenPrice  string        `json:"breakEvenPrice"`
	OpenTime        FlexibleInt64 `json:"openTime"`
	UpdatedTime     FlexibleInt64 `json:"updatedTime"`
	PositionStatus  string        `json:"positionStatus"`
	IsReduceOnly    bool          `json:"isReduceOnly"`
}

type PositionResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		NextPageCursor string     `json:"nextPageCursor"`
		Category       string     `json:"category"`
		List           []Position `json:"list"`
	} `json:"result"`
}
