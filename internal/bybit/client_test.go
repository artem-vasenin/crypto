package bybit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// TestDoGET проверяет общий конверт Bybit и декодирование result.
func TestDoGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"retCode": 0,
			"retMsg":  "OK",
			"result":  map[string]any{"value": 42},
		})
	}))
	defer server.Close()
	c := NewClient(ClientConfig{BaseURL: server.URL, HTTPTimeout: time.Second})
	var got struct {
		Value int `json:"value"`
	}
	if err := c.doGET(context.Background(), "/test", url.Values{}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Value != 42 {
		t.Fatalf("value = %d, want 42", got.Value)
	}
}

func TestOrderBook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"retCode": 0,
			"retMsg":  "OK",
			"result": map[string]any{
				"b": [][]string{{"10", "2"}, {"9", "1"}},
				"a": [][]string{{"11", "1"}, {"12", "1"}},
			},
		})
	}))
	defer server.Close()
	c := NewClient(ClientConfig{BaseURL: server.URL, HTTPTimeout: time.Second})
	book, err := c.OrderBook(context.Background(), "TESTUSDT", 50)
	if err != nil {
		t.Fatal(err)
	}
	if book.BidNotional != 29 {
		t.Fatalf("bid notional = %.2f, want 29", book.BidNotional)
	}
	if book.AskNotional != 23 {
		t.Fatalf("ask notional = %.2f, want 23", book.AskNotional)
	}
	if book.ImbalancePct <= 10 || book.ImbalancePct >= 12 {
		t.Fatalf("imbalance = %.2f", book.ImbalancePct)
	}
	if book.BidAskRatio <= 1.25 || book.BidAskRatio >= 1.27 {
		t.Fatalf("bid/ask ratio = %.2f", book.BidAskRatio)
	}
	if book.Levels != 4 {
		t.Fatalf("levels = %d, want 4", book.Levels)
	}
}
