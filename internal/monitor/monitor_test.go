package monitor

import (
	"testing"

	"bybit_monitor/internal/bybit"

	"github.com/stretchr/testify/require"
)

func TestMonitor_UpdatePosition_AddsNewPosition(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		Symbol: "B3USDT",
		Side:   "Sell",
		Size:   "21000",
	}

	monitor.UpdatePosition(position)

	positions := monitor.Positions()

	require.Len(t, positions, 1)
	require.Equal(t, position, positions[0])
}

func TestMonitor_UpdatePosition_UpdatesExistingPosition(t *testing.T) {
	monitor := New()

	first := bybit.Position{
		Symbol: "B3USDT",
		Side:   "Sell",
		Size:   "21600",
	}

	monitor.UpdatePosition(first)

	updated := bybit.Position{
		Symbol:        "B3USDT",
		Side:          "Sell",
		Size:          "21000",
		UnrealisedPnl: "-0.1554",
	}

	monitor.UpdatePosition(updated)

	positions := monitor.Positions()

	require.Len(t, positions, 1)
	require.Equal(t, updated, positions[0])
}

func TestMonitor_UpdatePosition_RemovesClosedPosition(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		Symbol: "B3USDT",
		Side:   "Sell",
		Size:   "21000",
	}

	monitor.UpdatePosition(position)

	closed := bybit.Position{
		Symbol: "B3USDT",
		Side:   "",
		Size:   "0",
	}

	monitor.UpdatePosition(closed)

	positions := monitor.Positions()

	require.Empty(t, positions)
}

func TestMonitor_UpdatePosition_DoesNotCreateDuplicate(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		Symbol: "B3USDT",
		Side:   "Sell",
		Size:   "21000",
	}

	monitor.UpdatePosition(position)
	monitor.UpdatePosition(position)

	positions := monitor.Positions()

	require.Len(t, positions, 1)
	require.Equal(t, position, positions[0])
}

func TestMonitor_UpdatePosition_MultiplePositions(t *testing.T) {
	monitor := New()

	b3 := bybit.Position{
		Symbol: "B3USDT",
		Side:   "Sell",
		Size:   "21000",
	}

	btc := bybit.Position{
		Symbol: "BTCUSDT",
		Side:   "Buy",
		Size:   "0.001",
	}

	monitor.UpdatePosition(b3)
	monitor.UpdatePosition(btc)

	positions := monitor.Positions()

	require.Len(t, positions, 2)
	require.Equal(t, b3, positions[0])
	require.Equal(t, btc, positions[1])
}

func TestMonitor_UpdatePosition_UpdatesOnlyMatchingPosition(t *testing.T) {
	monitor := New()

	b3 := bybit.Position{
		Symbol:        "B3USDT",
		Side:          "Sell",
		Size:          "21000",
		UnrealisedPnl: "-0.15",
	}

	btc := bybit.Position{
		Symbol:        "BTCUSDT",
		Side:          "Buy",
		Size:          "0.001",
		UnrealisedPnl: "1.25",
	}

	monitor.UpdatePosition(b3)
	monitor.UpdatePosition(btc)

	updatedB3 := bybit.Position{
		Symbol:        "B3USDT",
		Side:          "Sell",
		Size:          "20000",
		UnrealisedPnl: "-0.20",
	}

	monitor.UpdatePosition(updatedB3)

	positions := monitor.Positions()

	require.Len(t, positions, 2)

	require.Equal(t, updatedB3, positions[0])
	require.Equal(t, btc, positions[1])
}
