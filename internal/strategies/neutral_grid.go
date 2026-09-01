// internal/strategies/neutral_grid.go
package strategies

import "universal-bybit-screener/models"

// NeutralGrid оценивает флетовые боковики для спотовых/сеточных ботов
type NeutralGrid struct{}

func (NeutralGrid) Name() string { return "neutral-grid" }

func (NeutralGrid) Evaluate(c *models.Candidate) models.StrategyResult {
	st1 := c.Structure["1h"]
	st4 := c.Structure["4h"]

	// Hard Gate: Восходящий/падающий тренд на 1h и 4h
	if (st1.HighState == "HH" && st1.LowState == "HL" && st4.HighState == "HH") ||
		(st1.HighState == "LH" && st1.LowState == "LL" && st4.HighState == "LH") {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "strong trend detected on 1h/4h"}
	}

	// Hard Gate: Расширяющийся клин (HH + LL) — критический риск Impermanent Loss
	if st1.HighState == "HH" && st1.LowState == "LL" {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "broadening formation (HH+LL) detected - high risk of IL"}
	}

	// Hard Gate: Отсутствие сформированного канала
	if c.Levels.NearestResistance == 0 || c.Levels.NearestSupport == 0 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "no complete support/resistance range"}
	}

	// Hard Gate: Недостаточный диапазон относительно ATR (риск мгновенного выбивания сетки)
	if c.Levels.RangeToATR1h < 1.5 {
		return models.StrategyResult{Score: 0, Status: "reject", Reason: "range width insufficient against 1h volatility"}
	}

	score := 50.0

	// Положение цены ближе к центру канала — идеальная точка старта
	if c.Levels.RangePositionPct >= 35 && c.Levels.RangePositionPct <= 65 {
		score += 25
	} else if c.Levels.RangePositionPct < 20 || c.Levels.RangePositionPct > 80 {
		score -= 20
	}

	// Коридор 3% - 15% оптимален для забора сеточной сетки
	if c.Levels.RangeWidthPct >= 3 && c.Levels.RangeWidthPct <= 15 {
		score += 15
	}

	if c.Indicators.RSI1h >= 40 && c.Indicators.RSI1h <= 60 {
		score += 10
	}

	score = clamp(score)

	return models.StrategyResult{
		Score:  score,
		Status: status(score),
		Reason: "stable range with multi-TF volatility protection",
	}
}
