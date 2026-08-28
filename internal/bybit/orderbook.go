package bybit

import (
	"context"
	"net/url"
	"sc/models"
)

// OrderBook получает верхние уровни стакана. Данные публичные, авторизация не нужна.
func (c *Client) OrderBook(ctx context.Context, symbol string, limit int) (models.OrderBookMetrics, error) {
	q := url.Values{"category": {linear}, "symbol": {symbol}, "limit": {fmtInt(limit)}}
	// Bybit отдаёт уровни как массивы [price, size]. Для простоты ниже
	// используется RawMessage-подобная форма через [][]string.
	var raw struct {
		B [][]string `json:"b"`
		A [][]string `json:"a"`
	}
	if err := c.doGET(ctx, "/v5/market/orderbook", q, &raw); err != nil {
		return models.OrderBookMetrics{}, err
	}
	out := models.OrderBookMetrics{Levels: len(raw.B) + len(raw.A)}
	for _, level := range raw.B {
		if len(level) >= 2 {
			out.BidNotional += f(level[0]) * f(level[1])
		}
	}
	for _, level := range raw.A {
		if len(level) >= 2 {
			out.AskNotional += f(level[0]) * f(level[1])
		}
	}
	total := out.BidNotional + out.AskNotional
	if total > 0 {
		out.ImbalancePct = (out.BidNotional - out.AskNotional) / total * 100
	}
	return out, nil
}
