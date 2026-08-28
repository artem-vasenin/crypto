package strategies

import (
	"fmt"
	"strings"
	"universal-bybit-screener/models"
)

// Strategy описывает единый интерфейс всех режимов скриннера.
type Strategy interface {
	Name() string
	Evaluate(models.MarketData, models.Indicators, map[string]models.Structure, models.Levels) models.StrategyResult
}

// Names возвращает канонический порядок стратегий. Он используется и анализом,
// и тестами, поэтому новая стратегия не может случайно забыться в одном месте.
func Names() []string { return []string{"short-grid", "short", "long-grid", "long", "neutral-grid"} }

func New(name string) (Strategy, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "short-grid":
		return ShortGrid{}, nil
	case "short":
		return Short{}, nil
	case "long-grid":
		return LongGrid{}, nil
	case "long":
		return Long{}, nil
	case "neutral-grid":
		return NeutralGrid{}, nil
	}
	return nil, fmt.Errorf("unknown strategy %q", name)
}
func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
func status(score float64) string {
	switch {
	case score >= 75:
		return "consider"
	case score >= 55:
		return "watch"
	case score >= 35:
		return "risky"
	default:
		return "avoid"
	}
}
