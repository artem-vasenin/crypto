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
