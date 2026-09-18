package universe

import (
	"candidate-screener/internal/domain"
	"testing"
)

func TestSelectLiquidity(t *testing.T) {
	i := []domain.Instrument{{Symbol: "A"}, {Symbol: "B"}}
	m := map[string]domain.Ticker{"A": {LastPrice: 1, Turnover24h: 10}, "B": {LastPrice: 1, Turnover24h: 100}}
	o := Select(i, m, 50)
	if len(o) != 1 || o[0].Symbol != "B" {
		t.Fatalf("неверный universe: %#v", o)
	}
}
