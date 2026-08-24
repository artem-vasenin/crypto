package monitor

import "bybit_monitor/internal/bybit"

type State struct {
	Positions []bybit.Position
}

type Monitor struct {
	state State
}

type PositionChange struct {
	Field string
	From  string
	To    string
}

type PositionEventType string

const (
	PositionOpened  PositionEventType = "opened"
	PositionUpdated PositionEventType = "updated"
	PositionClosed  PositionEventType = "closed"
)

type PositionEvent struct {
	Type     PositionEventType
	Previous *bybit.Position
	Current  *bybit.Position
	Changes  []PositionChange
}

func New() *Monitor {
	return &Monitor{}
}

func (m *Monitor) UpdatePositions(positions []bybit.Position) {
	m.state.Positions = positions
}

func (m *Monitor) Positions() []bybit.Position {
	return m.state.Positions
}

func (m *Monitor) Position(symbol string, positionIdx int) (bybit.Position, bool) {
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

func (m *Monitor) UpdatePosition(position bybit.Position) PositionEvent {
	for i, current := range m.state.Positions {
		if current.Symbol != position.Symbol {
			continue
		}

		if current.PositionIdx != position.PositionIdx {
			continue
		}

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

		previous := current

		m.state.Positions[i] = position

		return PositionEvent{
			Type:     PositionUpdated,
			Previous: &previous,
			Current:  &position,
			Changes:  detectChanges(previous, position),
		}
	}

	if position.Size != "0" {
		m.state.Positions = append(m.state.Positions, position)

		return PositionEvent{
			Type:    PositionOpened,
			Current: &position,
		}
	}

	return PositionEvent{}
}

func detectChanges(
	previous bybit.Position,
	current bybit.Position,
) []PositionChange {
	var changes []PositionChange

	if previous.TakeProfit != current.TakeProfit {
		changes = append(changes, PositionChange{
			Field: "TakeProfit",
			From:  previous.TakeProfit,
			To:    current.TakeProfit,
		})
	}

	if previous.StopLoss != current.StopLoss {
		changes = append(changes, PositionChange{
			Field: "StopLoss",
			From:  previous.StopLoss,
			To:    current.StopLoss,
		})
	}

	if previous.Size != current.Size {
		changes = append(changes, PositionChange{
			Field: "Size",
			From:  previous.Size,
			To:    current.Size,
		})
	}

	if previous.MarkPrice != current.MarkPrice {
		changes = append(changes, PositionChange{
			Field: "MarkPrice",
			From:  previous.MarkPrice,
			To:    current.MarkPrice,
		})
	}

	if previous.UnrealisedPnl != current.UnrealisedPnl {
		changes = append(changes, PositionChange{
			Field: "UnrealisedPnl",
			From:  previous.UnrealisedPnl,
			To:    current.UnrealisedPnl,
		})
	}

	if previous.Leverage != current.Leverage {
		changes = append(changes, PositionChange{
			Field: "Leverage",
			From:  previous.Leverage,
			To:    current.Leverage,
		})
	}

	return changes
}
