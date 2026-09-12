package execution

import (
	"math"
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
			"long": {Decision: models.StrategyDecision{Eligible: true}},
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

func TestShortTakeProfitDistanceIsPositive(t *testing.T) {
	tp := CalculateDynamicTakeProfit("Sell", 100, 105, 2, 0.01)
	if tp >= 100 {
		t.Fatalf("expected short TP below entry, got %.2f", tp)
	}
	distance := math.Abs(tp-100) / 100 * 100
	if distance <= 0 {
		t.Fatalf("expected positive TP distance, got %.4f%%", distance)
	}
}

func TestCalculateRiskLevelsForShort(t *testing.T) {
	engine := &Engine{cfg: models.BotConfig{
		MakerFeeRate:    0.0002,
		TakerFeeRate:    0.00055,
		ExtraCostPct:    0.02,
		MaxStopLossPct:  5,
		MinNetProfitPct: 0.5,
	}}
	candidate := models.Candidate{}
	candidate.Indicators.ATR1h = 1
	candidate.Indicators.ATR1hPct = 1
	candidate.Levels.NearestResistance = 102

	sl, tp, _, err := engine.calculateRiskLevels("Sell", 100, candidate, 0.01)
	if err != nil {
		t.Fatalf("short risk levels should pass: %v", err)
	}
	if sl <= 100 || tp >= 100 {
		t.Fatalf("invalid short levels: SL=%.2f TP=%.2f", sl, tp)
	}
}
