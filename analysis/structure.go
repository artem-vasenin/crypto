package analysis

import (
	"cs/model"
)

// FindPivotHighs ищет локальные максимумы.
//
// left/right определяют,
// сколько свечей слева и справа должно быть меньше.
//
// Например:
//
// left = 2
// right = 2
//
// означает:
//
//	предыдущие 2 свечи
//	       ↓
//	    HIGH
//	       ↑
//	следующие 2 свечи
//
// должны иметь High ниже этого значения.
func FindPivotHighs(
	candles []model.Candle,
	left int,
	right int,
) []model.Pivot {

	if len(candles) < left+right+1 {
		return nil
	}

	result := make(
		[]model.Pivot,
		0,
	)

	for i := left; i < len(candles)-right; i++ {

		isPivot := true

		currentHigh := candles[i].High

		// Проверяем свечи слева.
		for j := i - left; j < i; j++ {

			if candles[j].High >= currentHigh {
				isPivot = false
				break
			}
		}

		if !isPivot {
			continue
		}

		// Проверяем свечи справа.
		for j := i + 1; j <= i+right; j++ {

			if candles[j].High >= currentHigh {
				isPivot = false
				break
			}
		}

		if isPivot {
			result = append(
				result,
				model.Pivot{
					Time:  candles[i].Time,
					Price: currentHigh,
				},
			)
		}
	}

	return result
}

// FindPivotLows ищет локальные минимумы.
func FindPivotLows(
	candles []model.Candle,
	left int,
	right int,
) []model.Pivot {

	if len(candles) < left+right+1 {
		return nil
	}

	result := make(
		[]model.Pivot,
		0,
	)

	for i := left; i < len(candles)-right; i++ {

		isPivot := true

		currentLow := candles[i].Low

		for j := i - left; j < i; j++ {

			if candles[j].Low <= currentLow {
				isPivot = false
				break
			}
		}

		if !isPivot {
			continue
		}

		for j := i + 1; j <= i+right; j++ {

			if candles[j].Low <= currentLow {
				isPivot = false
				break
			}
		}

		if isPivot {
			result = append(
				result,
				model.Pivot{
					Time:  candles[i].Time,
					Price: currentLow,
				},
			)
		}
	}

	return result
}

// CompareHighStructure сравнивает два последних pivot high.
//
// current > previous → HH
// current < previous → LH
func CompareHighStructure(
	pivots []model.Pivot,
) (
	string,
	float64,
	float64,
	float64,
) {

	if len(pivots) < 2 {
		return "NONE", 0, 0, 0
	}

	previous := pivots[len(pivots)-2]
	current := pivots[len(pivots)-1]

	change := (current.Price/previous.Price - 1) * 100

	if current.Price > previous.Price {
		return "HH", change, previous.Price, current.Price
	}

	return "LH", change, previous.Price, current.Price
}

// CompareLowStructure сравнивает два последних pivot low.
//
// current > previous → HL
// current < previous → LL
func CompareLowStructure(
	pivots []model.Pivot,
) (
	string,
	float64,
	float64,
	float64,
) {

	if len(pivots) < 2 {
		return "NONE", 0, 0, 0
	}

	previous := pivots[len(pivots)-2]
	current := pivots[len(pivots)-1]

	change := (current.Price/previous.Price - 1) * 100

	if current.Price > previous.Price {
		return "HL", change, previous.Price, current.Price
	}

	return "LL", change, previous.Price, current.Price
}

// BuildStructure собирает всю структуру одного таймфрейма.
func BuildStructure(
	interval string,
	candles []model.Candle,
	currentPrice float64,
) model.Structure {

	if len(candles) == 0 {
		return model.Structure{
			Interval: interval,
		}
	}

	high := 0.0
	low := candles[0].Low

	for _, candle := range candles {

		if candle.High > high {
			high = candle.High
		}

		if candle.Low < low {
			low = candle.Low
		}
	}

	rangePercent := 0.0

	if low > 0 {
		rangePercent =
			(high/low - 1) * 100
	}

	fromHigh := 0.0

	if high > 0 {
		fromHigh =
			(currentPrice/high - 1) * 100
	}

	position := 0.0

	if high != low {
		position =
			(currentPrice - low) /
				(high - low) *
				100
	}

	// Для 4H нам не нужно искать pivot через
	// слишком большое количество свечей.
	//
	// 2 слева + 2 справа — хороший базовый вариант.
	pivotHighs :=
		FindPivotHighs(candles, 2, 2)

	pivotLows :=
		FindPivotLows(candles, 2, 2)

	// Сохраняем только последние 5 pivot'ов.
	if len(pivotHighs) > 5 {
		pivotHighs =
			pivotHighs[len(pivotHighs)-5:]
	}

	if len(pivotLows) > 5 {
		pivotLows =
			pivotLows[len(pivotLows)-5:]
	}

	highStructure,
		highChange,
		previousHigh,
		currentHigh :=
		CompareHighStructure(pivotHighs)

	lowStructure,
		lowChange,
		previousLow,
		currentLow :=
		CompareLowStructure(pivotLows)

	return model.Structure{
		Interval: interval,

		High: high,
		Low:  low,

		RangePercent:    rangePercent,
		FromHighPercent: fromHigh,
		PositionPercent: position,

		PivotHighs: pivotHighs,
		PivotLows:  pivotLows,

		HighStructure:        highStructure,
		HighStructurePercent: highChange,
		PreviousHigh:         previousHigh,
		CurrentHigh:          currentHigh,

		LowStructure:        lowStructure,
		LowStructurePercent: lowChange,
		PreviousLow:         previousLow,
		CurrentLow:          currentLow,
	}
}

// FindNearestResistance ищет ближайший pivot high
// выше текущей цены.
//
// Это пока простая версия.
// Позже сделаем полноценные зоны сопротивления,
// объединяя близкие уровни.
func FindNearestResistance(
	structures ...model.Structure,
) float64 {

	var resistance float64

	for _, structure := range structures {

		for _, pivot := range structure.PivotHighs {

			if pivot.Price <= 0 {
				continue
			}

			// Нас интересуют только уровни выше текущего
			// значения, поэтому current price здесь
			// будет отфильтрован вызывающей функцией.
			if resistance == 0 ||
				pivot.Price < resistance {

				resistance = pivot.Price
			}
		}
	}

	return resistance
}
