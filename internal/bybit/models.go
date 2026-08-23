package bybit

type Position struct {
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Size          string `json:"size"`
	Leverage      string `json:"leverage"`
	AvgPrice      string `json:"avgPrice"`
	MarkPrice     string `json:"markPrice"`
	LiqPrice      string `json:"liqPrice"`
	PositionValue string `json:"positionValue"`
	UnrealisedPnl string `json:"unrealisedPnl"`
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
