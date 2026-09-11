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
