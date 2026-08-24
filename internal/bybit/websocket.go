package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"bybit_monitor/internal/config"

	"github.com/gorilla/websocket"
)

// WebSocketClient отвечает за private WebSocket Bybit.
//
// В отличие от старого варианта:
//
//	privateWebSocketURL = "..."
//
// URL теперь приходит из config.
//
// Это позволяет использовать один и тот же код
// для Testnet и Mainnet.
type WebSocketClient struct {
	url       string
	apiKey    string
	apiSecret string

	mu   sync.Mutex
	conn *websocket.Conn
}

// wsAuthRequest — сообщение авторизации.
//
// Формат Bybit:
//
//	{
//	  "op": "auth",
//	  "args": [
//	    "api_key",
//	    expires,
//	    "signature"
//	  ]
//	}
type wsAuthRequest struct {
	Op   string        `json:"op"`
	Args []interface{} `json:"args"`
}

// wsSubscribeRequest — подписка на topic.
type wsSubscribeRequest struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

// wsMessageHeader нужен для быстрой идентификации сообщения.
//
// Мы сначала читаем только topic/op,
// а уже потом разбираем конкретный payload.
type wsMessageHeader struct {
	Topic string `json:"topic"`
	Op    string `json:"op"`
}

// WebSocketMessage — наша внутренняя модель сообщения.
type WebSocketMessage struct {
	Topic string
	Op    string
	Data  []wsPosition
}

// wsPositionMessage — сообщение private topic position.
type wsPositionMessage struct {
	Topic        string       `json:"topic"`
	CreationTime int64        `json:"creationTime"`
	Data         []wsPosition `json:"data"`
}

// wsPosition — модель позиции именно WebSocket API.
//
// Она похожа на Position из REST,
// но названия некоторых полей отличаются.
//
// Например:
//
//	entryPrice -> AvgPrice
//
// Это мы преобразуем в ToPosition().
type wsPosition struct {
	PositionIdx int    `json:"positionIdx"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`

	Size       string `json:"size"`
	EntryPrice string `json:"entryPrice"`
	Leverage   string `json:"leverage"`
	MarkPrice  string `json:"markPrice"`
	LiqPrice   string `json:"liqPrice"`

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

// ToPosition переводит WebSocket-модель
// в нашу единую модель Position.
//
// После этого Monitor вообще не знает,
// откуда пришла позиция:
// REST или WebSocket.
func (p wsPosition) ToPosition() Position {
	return Position{
		PositionIdx: p.PositionIdx,

		Symbol: p.Symbol,
		Side:   p.Side,

		Size:     p.Size,
		Leverage: p.Leverage,

		AvgPrice:  p.EntryPrice,
		MarkPrice: p.MarkPrice,
		LiqPrice:  p.LiqPrice,

		PositionValue:   p.PositionValue,
		PositionBalance: p.PositionBalance,

		PositionIM: p.PositionIM,
		PositionMM: p.PositionMM,

		UnrealisedPnl:  p.UnrealisedPnl,
		CumRealisedPnl: p.CumRealisedPnl,
		CurRealisedPnl: p.CurRealisedPnl,

		TakeProfit:   p.TakeProfit,
		StopLoss:     p.StopLoss,
		TrailingStop: p.TrailingStop,

		BreakEvenPrice: p.BreakEvenPrice,

		OpenTime:    p.OpenTime,
		UpdatedTime: p.UpdatedTime,

		PositionStatus: p.PositionStatus,

		IsReduceOnly: p.IsReduceOnly,
	}
}

// NewWebSocketClient создаёт WebSocket-клиент.
//
// Важный момент:
//
// Здесь соединение ещё НЕ устанавливается.
//
// Мы только сохраняем настройки.
//
// Само подключение выполняется в connect().
func NewWebSocketClient(cfg config.ByBit) *WebSocketClient {
	return &WebSocketClient{
		url:       cfg.PrivateWebSocketURL,
		apiKey:    cfg.APIKey,
		apiSecret: cfg.Secret,
	}
}

// connect устанавливает физическое WebSocket-соединение.
func (c *WebSocketClient) connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(
		c.url,
		nil,
	)

	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	return nil
}

// Close закрывает WebSocket.
//
// Mutex нужен потому,
// что одновременно может работать heartbeat goroutine
// и основной read loop.
func (c *WebSocketClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()

	c.conn = nil

	return err
}

// Authenticate авторизует текущий WebSocket.
//
// Формула подписи Bybit:
//
//	GET/realtime + expires
func (c *WebSocketClient) Authenticate() error {
	expires := time.Now().UnixMilli() + 5000

	payload := "GET/realtime" +
		strconv.FormatInt(expires, 10)

	signature := generateSignature(
		c.apiSecret,
		payload,
	)

	request := wsAuthRequest{
		Op: "auth",

		Args: []interface{}{
			c.apiKey,
			expires,
			signature,
		},
	}

	return c.writeJSON(request)
}

// Subscribe подписывает соединение на нужные topics.
func (c *WebSocketClient) Subscribe(
	topics ...string,
) error {
	request := wsSubscribeRequest{
		Op:   "subscribe",
		Args: topics,
	}

	return c.writeJSON(request)
}

// writeJSON безопасно пишет JSON в WebSocket.
//
// Все записи проходят через mutex.
//
// Это важно, потому что:
//
//	main goroutine
//	heartbeat goroutine
//
// потенциально могут одновременно писать в одно соединение.
func (c *WebSocketClient) writeJSON(
	value interface{},
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("websocket is not connected")
	}

	return c.conn.WriteJSON(value)
}

// ReadMessage читает одно WebSocket-сообщение.
func (c *WebSocketClient) ReadMessage() ([]byte, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf(
			"websocket is not connected",
		)
	}

	_, message, err := conn.ReadMessage()

	if err != nil {
		return nil, err
	}

	return message, nil
}

// StartHeartbeat запускает heartbeat.
//
// Bybit рекомендует ping примерно каждые 20 секунд.
func (c *WebSocketClient) StartHeartbeat(
	ctx context.Context,
) {
	ticker := time.NewTicker(20 * time.Second)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				err := c.writeJSON(
					map[string]string{
						"op": "ping",
					},
				)

				if err != nil {
					log.Println(
						"websocket heartbeat:",
						err,
					)

					return
				}
			}
		}
	}()
}

// Run запускает основной WebSocket loop.
//
// Алгоритм:
//
//	connect
//	  ↓
//	authenticate
//	  ↓
//	subscribe
//	  ↓
//	heartbeat
//	  ↓
//	read messages
//	  ↓
//	connection lost?
//	  ↓
//	wait
//	  ↓
//	reconnect
//
// Это уже существенно ближе к production,
// чем наш первый учебный вариант.
func (c *WebSocketClient) Run(
	ctx context.Context,
	topics []string,
	handler func([]byte),
) error {
	reconnectDelay := 2 * time.Second

	for {
		// Если приложение уже остановили — выходим.
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
		}

		log.Println("connecting WebSocket...")

		err := c.connect()
		if err != nil {
			log.Println("websocket connect:", err)

			if !wait(ctx, reconnectDelay) {
				return ctx.Err()
			}

			continue
		}

		log.Println("WebSocket connected")

		// После каждого нового соединения
		// нужно заново авторизоваться.
		err = c.Authenticate()
		if err != nil {
			log.Println(
				"websocket authenticate:",
				err,
			)

			_ = c.Close()

			if !wait(ctx, reconnectDelay) {
				return ctx.Err()
			}

			continue
		}

		// И заново подписаться.
		err = c.Subscribe(topics...)
		if err != nil {
			log.Println(
				"websocket subscribe:",
				err,
			)

			_ = c.Close()

			if !wait(ctx, reconnectDelay) {
				return ctx.Err()
			}

			continue
		}

		// Отдельный context для heartbeat именно этого соединения.
		connectionCtx, cancel := context.WithCancel(ctx)

		c.StartHeartbeat(connectionCtx)

		// Сбрасываем задержку после успешного подключения.
		reconnectDelay = 2 * time.Second

		// Основной read loop.
		for {
			message, err := c.ReadMessage()

			if err != nil {
				cancel()

				log.Println(
					"websocket read:",
					err,
				)

				_ = c.Close()

				break
			}

			handler(message)

			// Проверяем, не попросили ли нас остановиться.
			select {
			case <-ctx.Done():
				cancel()
				_ = c.Close()

				return ctx.Err()

			default:
			}
		}

		// Небольшая защита от слишком частых reconnect.
		if reconnectDelay < 30*time.Second {
			reconnectDelay *= 2
		}
	}

}

// wait ждёт перед reconnect.
//
// Но если приложение остановлено,
// ждать весь reconnect delay уже бессмысленно.
func wait(
	ctx context.Context,
	delay time.Duration,
) bool {
	timer := time.NewTimer(delay)

	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false

	case <-timer.C:
		return true
	}
}

// ParseWebSocketMessage разбирает входящее сообщение.
//
// Сначала определяем topic.
//
// Только если это position,
// разбираем data.
func ParseWebSocketMessage(
	message []byte,
) (*WebSocketMessage, error) {
	var header wsMessageHeader

	err := json.Unmarshal(
		message,
		&header,
	)

	if err != nil {
		return nil, err
	}

	result := &WebSocketMessage{
		Topic: header.Topic,
		Op:    header.Op,
	}

	if header.Topic == "position" {
		var response wsPositionMessage

		err := json.Unmarshal(
			message,
			&response,
		)

		if err != nil {
			return nil, err
		}

		result.Data = response.Data
	}

	return result, nil
}
