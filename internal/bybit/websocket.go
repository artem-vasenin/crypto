package bybit

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	conn *websocket.Conn
}

type wsAuthRequest struct {
	Op   string        `json:"op"`
	Args []interface{} `json:"args"`
}

type wsSubscribeRequest struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

type wsMessageHeader struct {
	Topic string `json:"topic"`
	Op    string `json:"op"`
}

type WebSocketMessage struct {
	Topic string
	Op    string
	Data  []wsPosition
}

type wsPositionMessage struct {
	Topic        string       `json:"topic"`
	CreationTime int64        `json:"creationTime"`
	Data         []wsPosition `json:"data"`
}

type wsPosition struct {
	PositionIdx     int           `json:"positionIdx"`
	Symbol          string        `json:"symbol"`
	Side            string        `json:"side"`
	Size            string        `json:"size"`
	EntryPrice      string        `json:"entryPrice"`
	Leverage        string        `json:"leverage"`
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

func (p wsPosition) ToPosition() Position {
	return Position{
		PositionIdx:     p.PositionIdx,
		Symbol:          p.Symbol,
		Side:            p.Side,
		Size:            p.Size,
		Leverage:        p.Leverage,
		AvgPrice:        p.EntryPrice,
		MarkPrice:       p.MarkPrice,
		LiqPrice:        p.LiqPrice,
		PositionValue:   p.PositionValue,
		PositionBalance: p.PositionBalance,
		PositionIM:      p.PositionIM,
		PositionMM:      p.PositionMM,
		UnrealisedPnl:   p.UnrealisedPnl,
		CumRealisedPnl:  p.CumRealisedPnl,
		CurRealisedPnl:  p.CurRealisedPnl,
		TakeProfit:      p.TakeProfit,
		StopLoss:        p.StopLoss,
		TrailingStop:    p.TrailingStop,
		BreakEvenPrice:  p.BreakEvenPrice,
		OpenTime:        p.OpenTime,
		UpdatedTime:     p.UpdatedTime,
		PositionStatus:  p.PositionStatus,
		IsReduceOnly:    p.IsReduceOnly,
	}
}

func NewWebSocketClient(url string) (*WebSocketClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &WebSocketClient{
		conn: conn,
	}, nil
}

func (c *WebSocketClient) Close() error {
	return c.conn.Close()
}

func (c *WebSocketClient) ReadMessage() ([]byte, error) {
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	return message, nil
}

func (c *WebSocketClient) Authenticate(
	apiKey string,
	apiSecret string,
) error {
	expires := time.Now().UnixMilli() + 5000

	payload := "GET/realtime" + strconv.FormatInt(expires, 10)

	signature := generateSignature(
		apiSecret,
		payload,
	)

	request := wsAuthRequest{
		Op: "auth",
		Args: []interface{}{
			apiKey,
			expires,
			signature,
		},
	}

	return c.conn.WriteJSON(request)
}

func (c *WebSocketClient) Subscribe(topics ...string) error {
	request := wsSubscribeRequest{
		Op:   "subscribe",
		Args: topics,
	}

	return c.conn.WriteJSON(request)
}

func (c *WebSocketClient) StartHeartbeat() {
	ticker := time.NewTicker(20 * time.Second)

	go func() {
		defer ticker.Stop()

		for range ticker.C {
			err := c.conn.WriteJSON(map[string]string{
				"op": "ping",
			})

			if err != nil {
				return
			}
		}
	}()
}

func ParseWebSocketMessage(message []byte) (*WebSocketMessage, error) {
	var header wsMessageHeader

	err := json.Unmarshal(message, &header)
	if err != nil {
		return nil, err
	}

	result := &WebSocketMessage{
		Topic: header.Topic,
		Op:    header.Op,
	}

	if header.Topic == "position" {
		var response wsPositionMessage

		err := json.Unmarshal(message, &response)
		if err != nil {
			return nil, err
		}

		result.Data = response.Data
	}

	return result, nil
}
