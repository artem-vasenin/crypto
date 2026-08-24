package monitor

import "bybit_monitor/internal/bybit"

// State содержит текущее состояние монитора.
//
// Пока состояние очень простое:
//
//	Positions
//
// Позже сюда можно будет добавить:
//
//	Orders
//	Bots
//	Balance
//	Executions
//	Statistics
//
// Но пока этого делать не надо.
type State struct {
	Positions []bybit.Position
}

// Monitor отвечает за хранение и изменение состояния.
//
// Важный архитектурный принцип:
//
// Monitor не знает ничего о:
//
//	HTTP
//	WebSocket
//	Bybit
//	JSON
//	логировании
//
// Он получает Position и работает только с ней.
type Monitor struct {
	state State
}

// PositionChange описывает изменение конкретного поля.
//
// Например:
//
//	TakeProfit: 0.03891 -> 0.03941
type PositionChange struct {
	Field string
	From  string
	To    string
}

// PositionEventType описывает тип события.
type PositionEventType string

const (
	PositionOpened  PositionEventType = "opened"
	PositionUpdated PositionEventType = "updated"
	PositionClosed  PositionEventType = "closed"
)

// PositionEvent — результат обработки новой позиции.
//
// Previous:
//
//	какое состояние было до изменения
//
// Current:
//
//	какое состояние стало после изменения
//
// Changes:
//
//	конкретные изменившиеся поля
type PositionEvent struct {
	Type     PositionEventType
	Previous *bybit.Position
	Current  *bybit.Position
	Changes  []PositionChange
}

// New создаёт новый Monitor.
//
// State специально не принимаем параметром,
// потому что пока Monitor всегда начинается с пустого состояния.
func New() *Monitor {
	return &Monitor{}
}

// UpdatePositions полностью заменяет текущее состояние.
//
// Это используется при первоначальной синхронизации REST.
func (m *Monitor) UpdatePositions(
	positions []bybit.Position,
) {
	m.state.Positions = positions
}

// Positions возвращает все текущие позиции.
func (m *Monitor) Positions() []bybit.Position {
	return m.state.Positions
}

// Position ищет конкретную позицию.
//
// Одного Symbol недостаточно.
//
// В hedge mode у одного символа могут существовать
// разные позиции, поэтому учитываем:
//
//	Symbol
//	PositionIdx
func (m *Monitor) Position(
	symbol string,
	positionIdx int,
) (bybit.Position, bool) {
	for _, position := range m.state.Positions {
		if position.Symbol != symbol {
			continue
		}

		if position.PositionIdx != positionIdx {
			continue
		}

		return position, true
	}

	return bybit.Position{}, false
}

// UpdatePosition применяет новое состояние позиции.
//
// Возможны три ситуации:
//
//  1. позиции раньше не было
//     -> opened
//
//  2. позиция была
//     -> updated
//
//  3. пришёл size == 0
//     -> closed
func (m *Monitor) UpdatePosition(
	position bybit.Position,
) PositionEvent {
	for i, current := range m.state.Positions {
		if current.Symbol != position.Symbol {
			continue
		}

		if current.PositionIdx != position.PositionIdx {
			continue
		}

		// Bybit сообщает о закрытии позиции
		// через size == "0".
		if position.Size == "0" {
			previous := current

			m.state.Positions = append(
				m.state.Positions[:i],
				m.state.Positions[i+1:]...,
			)

			return PositionEvent{
				Type:     PositionClosed,
				Previous: &previous,
			}
		}

		// Сохраняем старое состояние,
		// чтобы потом определить изменения.
		previous := current

		m.state.Positions[i] = position

		return PositionEvent{
			Type:     PositionUpdated,
			Previous: &previous,
			Current:  &position,
			Changes: detectChanges(
				previous,
				position,
			),
		}
	}

	// Позиции раньше не существовало.
	//
	// Но size == 0 не является открытием.
	if position.Size != "0" {
		m.state.Positions = append(
			m.state.Positions,
			position,
		)

		return PositionEvent{
			Type:    PositionOpened,
			Current: &position,
		}
	}

	// Теоретически сюда можно попасть,
	// если получили закрытие позиции,
	// которой у нас вообще не было.
	return PositionEvent{}
}

// detectChanges сравнивает две позиции.
//
// Здесь мы специально не сравниваем абсолютно все поля.
//
// MarkPrice и UnrealisedPnl меняются постоянно,
// поэтому они нам интересны.
//
// А некоторые служебные поля Bybit
// пока не имеют практической ценности для вывода.
func detectChanges(
	previous bybit.Position,
	current bybit.Position,
) []PositionChange {
	var changes []PositionChange

	if previous.TakeProfit != current.TakeProfit {
		changes = append(
			changes,
			PositionChange{
				Field: "TakeProfit",
				From:  previous.TakeProfit,
				To:    current.TakeProfit,
			},
		)
	}

	if previous.StopLoss != current.StopLoss {
		changes = append(
			changes,
			PositionChange{
				Field: "StopLoss",
				From:  previous.StopLoss,
				To:    current.StopLoss,
			},
		)
	}

	if previous.Size != current.Size {
		changes = append(
			changes,
			PositionChange{
				Field: "Size",
				From:  previous.Size,
				To:    current.Size,
			},
		)
	}

	if previous.MarkPrice != current.MarkPrice {
		changes = append(
			changes,
			PositionChange{
				Field: "MarkPrice",
				From:  previous.MarkPrice,
				To:    current.MarkPrice,
			},
		)
	}

	if previous.UnrealisedPnl != current.UnrealisedPnl {
		changes = append(
			changes,
			PositionChange{
				Field: "UnrealisedPnl",
				From:  previous.UnrealisedPnl,
				To:    current.UnrealisedPnl,
			},
		)
	}

	if previous.Leverage != current.Leverage {
		changes = append(
			changes,
			PositionChange{
				Field: "Leverage",
				From:  previous.Leverage,
				To:    current.Leverage,
			},
		)
	}

	return changes
}
