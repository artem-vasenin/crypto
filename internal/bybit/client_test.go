package bybit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestTickers проверяет нормализацию ответа массового endpoint без обращения в интернет.
func TestTickers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"list":[{"symbol":"AAAUSDT","lastPrice":"2","turnover24h":"1000","volume24h":"500","fundingRate":"0.0001","openInterest":"10"}]}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, time.Second)
	m, err := c.Tickers(context.Background())
	if err != nil || m["AAAUSDT"].LastPrice != 2 {
		t.Fatalf("неверный результат: %#v, %v", m, err)
	}
}
