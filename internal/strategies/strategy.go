// internal/strategies/strategy.go
package strategies

import (
	"fmt"
	"math"
	"universal-bybit-screener/models"
)

type Strategy interface {
	Name() string
	Evaluate(c *models.Candidate) models.StrategyResult
}

func Names() []string {
	return []string{
		"long",
		"short",
	}
}

func New(name string) (Strategy, error) {
	switch name {
	case "long":
		return Long{}, nil
	case "short":
		return Short{}, nil
	default:
		return nil, fmt.Errorf("unknown strategy: %s", name)
	}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return math.Round(v)
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
