package execution

import (
	"testing"

	"universal-bybit-screener/models"
)

func TestCalculateDynamicStopLoss(t *testing.T) {
	sl := CalculateDynamicStopLoss("Buy", 100, 98, 2, 1.5, 0.1)
	if sl >= 100 || sl <= 0 {
		t.Fatalf("invalid long SL: %.4f", sl)
	}

	shortSL := CalculateDynamicStopLoss("Sell", 100, 102, 2, 1.5, 0.1)
	if shortSL <= 100 {
		t.Fatalf("invalid short SL: %.4f", shortSL)
	}

	missingPivot := CalculateDynamicStopLoss("Buy", 100, 0, 2, 1.5, 0.1)
	if missingPivot >= 100 || missingPivot <= 0 {
		t.Fatalf("missing pivot must fall back to ATR distance, got %.4f", missingPivot)
	}
}

func TestCalculatePositionQtyRejectsExchangeMinimum(t *testing.T) {
	_, err := CalculatePositionQty(3, 2, 100, 1, 1, 10, 1000)
	if err == nil {
		t.Fatal("expected minimum-notional rejection")
	}

	qty, err := CalculatePositionQty(3, 2, 10, 0.1, 0.1, 5, 1000)
	if err != nil {
		t.Fatalf("unexpected sizing error: %v", err)
	}
	if qty <= 0 || qty*10 < 5 {
		t.Fatalf("invalid sized quantity: %.4f", qty)
	}
}

func TestRiskValidation(t *testing.T) {
	if !ValidateStopLoss("Buy", 100, 97, 5, 2) {
		t.Fatal("expected valid long stop")
	}
	if ValidateStopLoss("Buy", 100, 99.5, 5, 2) {
		t.Fatal("stop inside ATR noise should be rejected")
	}
	if !ValidateTakeProfit("Buy", 100, 103, 2) {
		t.Fatal("expected valid long TP")
	}
}

func TestCalculateDynamicLeverage(t *testing.T) {
	candidate := models.Candidate{
		Strategies: map[string]models.StrategyResult{
			"long": {Score: 80},
		},
	}
	candidate.Indicators.ATR1hPct = 4
	if got := CalculateDynamicLeverage(candidate, "long", 2); got != 1 {
		t.Fatalf("expected reduced leverage in high volatility, got %d", got)
	}
}

func TestNewEngineTargetSide(t *testing.T) {
	cfg := models.BotConfig{ApiKey: "key", ApiSecret: "secret"}
	if got := NewEngine(cfg, "short").targetSide; got != "Sell" {
		t.Fatalf("short strategy must target Sell, got %q", got)
	}
	if got := NewEngine(cfg, "long").targetSide; got != "Buy" {
		t.Fatalf("long strategy must target Buy, got %q", got)
	}
}

func TestBuildPostOnlyOrderParams(t *testing.T) {
	params := buildPostOnlyOrderParams("BTCUSDT", "Sell", 10, 0.1, 100.1, 0.1, 102.2, 96.6)

	if params["side"] != "Sell" {
		t.Fatalf("expected Sell side, got %v", params["side"])
	}
	if params["timeInForce"] != "PostOnly" {
		t.Fatalf("expected PostOnly, got %v", params["timeInForce"])
	}
	if params["takeProfit"] != "96.6" {
		t.Fatalf("unexpected TP: %v", params["takeProfit"])
	}
	if params["stopLoss"] != "102.2" {
		t.Fatalf("unexpected SL: %v", params["stopLoss"])
	}
	if params["tpslMode"] != "Full" {
		t.Fatalf("expected Full TP/SL mode, got %v", params["tpslMode"])
	}
	if params["tpOrderType"] != "Market" || params["slOrderType"] != "Market" {
		t.Fatal("Full TP/SL must use Market trigger orders")
	}
}
