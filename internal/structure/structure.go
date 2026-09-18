package structure

import "candidate-screener/internal/domain"

// Classify использует подтверждённые локальные экстремумы и не трактует outside bar как чистый swing.
func Classify(c []domain.Candle, wing int) domain.StructureState {
	if len(c) < wing*2+5 {
		return domain.StructureUnknown
	}
	highs, lows := []float64{}, []float64{}
	for i := wing; i < len(c)-wing; i++ {
		hi, lo := true, true
		for j := i - wing; j <= i+wing; j++ {
			if j == i {
				continue
			}
			if c[j].High >= c[i].High {
				hi = false
			}
			if c[j].Low <= c[i].Low {
				lo = false
			}
		}
		if hi && lo {
			continue
		}
		if hi {
			highs = append(highs, c[i].High)
		}
		if lo {
			lows = append(lows, c[i].Low)
		}
	}
	if len(highs) < 2 || len(lows) < 2 {
		return domain.StructureUnknown
	}
	h1, h2 := highs[len(highs)-2], highs[len(highs)-1]
	l1, l2 := lows[len(lows)-2], lows[len(lows)-1]
	if h2 > h1 && l2 > l1 {
		return domain.StructureBull
	}
	if h2 < h1 && l2 < l1 {
		return domain.StructureBear
	}
	return domain.StructureMixed
}
