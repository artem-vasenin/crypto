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

func TestCalculateTrailingStopRequiresOneR(t *testing.T) {
	newSL, extreme, active, reason := CalculateTrailingStop("Buy", 100, 95, 102, 0, 1, 0.2, 0.1)
	if active || newSL != 0 || extreme != 0 || reason != "activation_not_reached" {
		t.Fatalf("unexpected inactive result: sl=%.4f extreme=%.4f active=%v reason=%s", newSL, extreme, active, reason)
	}

	newSL, extreme, active, reason = CalculateTrailingStop("Buy", 100, 95, 105, 0, 1, 0.2, 0.1)
	if !active || newSL != 103.9 || extreme != 105 || reason != "new_extreme" {
		t.Fatalf("unexpected active result: sl=%.4f extreme=%.4f active=%v reason=%s", newSL, extreme, active, reason)
	}
}

func TestCalculateTrailingStopIgnoresMicroMove(t *testing.T) {
	newSL, extreme, active, reason := CalculateTrailingStop("Sell", 100, 105, 94, 94, 1, 0.2, 0.01)
	if !active || newSL <= 0 || extreme != 94 || reason != "new_extreme" {
		t.Fatalf("unexpected first trailing result: sl=%.4f extreme=%.4f active=%v reason=%s", newSL, extreme, active, reason)
	}

	newSL, extreme, active, reason = CalculateTrailingStop("Sell", 100, newSL, 93.95, extreme, 1, 0.2, 0.01)
	if !active || extreme != 93.95 || reason != "sl_move_too_small" {
		t.Fatalf("micro move must not move SL: sl=%.4f extreme=%.4f active=%v reason=%s", newSL, extreme, active, reason)
	}
}

func TestCalculateTrailingStopNeverWorsensSL(t *testing.T) {
	newSL, _, active, reason := CalculateTrailingStop("Buy", 100, 104, 106, 106, 1, 0, 0.01)
	if !active || newSL <= 104 || reason != "new_extreme" {
		t.Fatalf("long trailing should improve SL: sl=%.4f active=%v reason=%s", newSL, active, reason)
	}

	newSL, _, active, reason = CalculateTrailingStop("Sell", 100, 96, 94, 94, 1, 0, 0.01)
	if !active || newSL >= 96 || reason != "new_extreme" {
		t.Fatalf("short trailing should improve SL: sl=%.4f active=%v reason=%s", newSL, active, reason)
	}
}

func TestHandlePositionUpdateMarksTargetSideAsManaged(t *testing.T) {
	engine := NewEngine(models.BotConfig{}, "short")
	engine.handlePositionUpdateWS(PositionUpdate{
		Symbol:     "TESTUSDT",
		Side:       "Sell",
		Size:       1,
		EntryPrice: 100,
	})

	engine.mu.Lock()
	pos := engine.positions["TESTUSDT"]
	engine.mu.Unlock()
	if pos == nil || !pos.Managed {
		t.Fatal("restored target-side position must be managed")
	}
}

func TestInitialStopLossKeepsATRDistance(t *testing.T) {
	sl := CalculateDynamicStopLoss("Sell", 100, 101, 1, 1.5, 0.01)
	if sl <= 100 {
		t.Fatalf("invalid short SL %.4f", sl)
	}
	distancePct := (sl - 100) / 100 * 100
	if distancePct < 1.5 {
		t.Fatalf("initial short SL must be at least 1.5 ATR away, got %.3f%%", distancePct)
	}
}
