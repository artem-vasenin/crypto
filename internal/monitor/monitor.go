package monitor

import "bybit_monitor/internal/bybit"

type State struct {
	Positions []bybit.Position
}

type Monitor struct {
	state State
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

func (m *Monitor) UpdatePosition(position bybit.Position) {
	for i, current := range m.state.Positions {
		if current.Symbol != position.Symbol {
			continue
		}

		if position.Size == "0" {
			m.state.Positions = append(
				m.state.Positions[:i],
				m.state.Positions[i+1:]...,
			)

			return
		}

		if current.Side == position.Side {
			m.state.Positions[i] = position

			return
		}
	}

	if position.Size != "0" {
		m.state.Positions = append(m.state.Positions, position)
	}
}
