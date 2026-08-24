package monitor

import (
	"testing"

	"bybit_monitor/internal/bybit"

	"github.com/stretchr/testify/require"
)

func TestMonitor_UpdatePosition_AddsNewPosition(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Sell",
		Size:        "21000",
	}

	event := monitor.UpdatePosition(position)

	positions := monitor.Positions()

	require.Len(t, positions, 1)
	require.Equal(t, position, positions[0])

	require.Equal(t, PositionOpened, event.Type)
	require.Nil(t, event.Previous)
	require.NotNil(t, event.Current)
	require.Equal(t, position, *event.Current)
	require.Empty(t, event.Changes)
}

func TestMonitor_UpdatePosition_UpdatesExistingPosition(t *testing.T) {
	monitor := New()

	first := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Sell",
		Size:        "21600",
	}

	monitor.UpdatePosition(first)

	updated := bybit.Position{
		PositionIdx:   0,
		Symbol:        "B3USDT",
		Side:          "Sell",
		Size:          "21000",
		UnrealisedPnl: "-0.1554",
	}

	event := monitor.UpdatePosition(updated)

	positions := monitor.Positions()

	require.Len(t, positions, 1)
	require.Equal(t, updated, positions[0])

	require.Equal(t, PositionUpdated, event.Type)
	require.NotNil(t, event.Previous)
	require.NotNil(t, event.Current)
	require.Equal(t, first, *event.Previous)
	require.Equal(t, updated, *event.Current)

	require.Len(t, event.Changes, 2)

	require.Equal(
		t,
		PositionChange{
			Field: "Size",
			From:  "21600",
			To:    "21000",
		},
		event.Changes[0],
	)

	require.Equal(
		t,
		PositionChange{
			Field: "UnrealisedPnl",
			From:  "",
			To:    "-0.1554",
		},
		event.Changes[1],
	)
}

func TestMonitor_UpdatePosition_RemovesClosedPosition(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Sell",
		Size:        "21000",
	}

	monitor.UpdatePosition(position)

	closed := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "",
		Size:        "0",
	}

	event := monitor.UpdatePosition(closed)

	positions := monitor.Positions()

	require.Empty(t, positions)

	require.Equal(t, PositionClosed, event.Type)
	require.NotNil(t, event.Previous)
	require.Nil(t, event.Current)
	require.Equal(t, position, *event.Previous)
	require.Empty(t, event.Changes)
}

func TestMonitor_UpdatePosition_DoesNotCreateDuplicate(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Sell",
		Size:        "21000",
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
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Sell",
		Size:        "21000",
	}

	btc := bybit.Position{
		PositionIdx: 0,
		Symbol:      "BTCUSDT",
		Side:        "Buy",
		Size:        "0.001",
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
		PositionIdx:   0,
		Symbol:        "B3USDT",
		Side:          "Sell",
		Size:          "21000",
		UnrealisedPnl: "-0.15",
	}

	btc := bybit.Position{
		PositionIdx:   0,
		Symbol:        "BTCUSDT",
		Side:          "Buy",
		Size:          "0.001",
		UnrealisedPnl: "1.25",
	}

	monitor.UpdatePosition(b3)
	monitor.UpdatePosition(btc)

	updatedB3 := bybit.Position{
		PositionIdx:   0,
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

func TestMonitor_UpdatePosition_UpdatesPositionWhenSideChanges(t *testing.T) {
	monitor := New()

	long := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Buy",
		Size:        "21000",
	}

	short := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Sell",
		Size:        "15000",
	}

	monitor.UpdatePosition(long)
	monitor.UpdatePosition(short)

	positions := monitor.Positions()

	require.Len(t, positions, 1)
	require.Equal(t, short, positions[0])
}

func TestMonitor_Position_ReturnsPosition(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx:   0,
		Symbol:        "B3USDT",
		Side:          "Sell",
		Size:          "21000",
		UnrealisedPnl: "-0.15",
	}

	monitor.UpdatePosition(position)

	found, ok := monitor.Position("B3USDT", 0)

	require.True(t, ok)
	require.Equal(t, position, found)
}

func TestMonitor_Position_ReturnsFalseWhenPositionNotFound(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx: 0,
		Symbol:      "B3USDT",
		Side:        "Sell",
		Size:        "21000",
	}

	monitor.UpdatePosition(position)

	found, ok := monitor.Position("BTCUSDT", 0)

	require.False(t, ok)
	require.Equal(t, bybit.Position{}, found)
}

func TestMonitor_Position_DistinguishesPositionIdx(t *testing.T) {
	monitor := New()

	buy := bybit.Position{
		PositionIdx: 1,
		Symbol:      "BTCUSDT",
		Side:        "Buy",
		Size:        "0.001",
	}

	sell := bybit.Position{
		PositionIdx: 2,
		Symbol:      "BTCUSDT",
		Side:        "Sell",
		Size:        "0.001",
	}

	monitor.UpdatePosition(buy)
	monitor.UpdatePosition(sell)

	foundBuy, ok := monitor.Position("BTCUSDT", 1)
	require.True(t, ok)
	require.Equal(t, buy, foundBuy)

	foundSell, ok := monitor.Position("BTCUSDT", 2)
	require.True(t, ok)
	require.Equal(t, sell, foundSell)
}

func TestMonitor_UpdatePosition_ReturnsOpenedEvent(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx: 0,
		Symbol:      "CHIPUSDT",
		Side:        "Buy",
		Size:        "300",
	}

	event := monitor.UpdatePosition(position)

	require.Equal(t, PositionOpened, event.Type)
	require.Nil(t, event.Previous)
	require.NotNil(t, event.Current)
	require.Equal(t, position, *event.Current)
	require.Empty(t, event.Changes)
}

func TestMonitor_UpdatePosition_ReturnsUpdatedEvent(t *testing.T) {
	monitor := New()

	first := bybit.Position{
		PositionIdx: 0,
		Symbol:      "CHIPUSDT",
		Side:        "Buy",
		Size:        "300",
		TakeProfit:  "0.03891",
	}

	monitor.UpdatePosition(first)

	updated := bybit.Position{
		PositionIdx: 0,
		Symbol:      "CHIPUSDT",
		Side:        "Buy",
		Size:        "300",
		TakeProfit:  "0.03941",
	}

	event := monitor.UpdatePosition(updated)

	require.Equal(t, PositionUpdated, event.Type)
	require.NotNil(t, event.Previous)
	require.NotNil(t, event.Current)

	require.Equal(t, first, *event.Previous)
	require.Equal(t, updated, *event.Current)

	require.Len(t, event.Changes, 1)

	require.Equal(
		t,
		PositionChange{
			Field: "TakeProfit",
			From:  "0.03891",
			To:    "0.03941",
		},
		event.Changes[0],
	)
}

func TestMonitor_UpdatePosition_ReturnsClosedEvent(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx: 0,
		Symbol:      "CHIPUSDT",
		Side:        "Buy",
		Size:        "300",
	}

	monitor.UpdatePosition(position)

	closed := bybit.Position{
		PositionIdx: 0,
		Symbol:      "CHIPUSDT",
		Side:        "",
		Size:        "0",
	}

	event := monitor.UpdatePosition(closed)

	require.Equal(t, PositionClosed, event.Type)
	require.NotNil(t, event.Previous)
	require.Nil(t, event.Current)
	require.Empty(t, event.Changes)
}

func TestMonitor_UpdatePosition_DetectsMultipleChanges(t *testing.T) {
	monitor := New()

	first := bybit.Position{
		PositionIdx:   0,
		Symbol:        "CHIPUSDT",
		Side:          "Buy",
		Size:          "300",
		MarkPrice:     "0.03263",
		TakeProfit:    "0.03891",
		StopLoss:      "0",
		UnrealisedPnl: "-0.033",
		Leverage:      "1",
	}

	monitor.UpdatePosition(first)

	updated := bybit.Position{
		PositionIdx:   0,
		Symbol:        "CHIPUSDT",
		Side:          "Buy",
		Size:          "250",
		MarkPrice:     "0.03280",
		TakeProfit:    "0.03941",
		StopLoss:      "0.03100",
		UnrealisedPnl: "0.012",
		Leverage:      "2",
	}

	event := monitor.UpdatePosition(updated)

	require.Equal(t, PositionUpdated, event.Type)
	require.Len(t, event.Changes, 6)

	require.Equal(t, PositionChange{
		Field: "TakeProfit",
		From:  "0.03891",
		To:    "0.03941",
	}, event.Changes[0])

	require.Equal(t, PositionChange{
		Field: "StopLoss",
		From:  "0",
		To:    "0.03100",
	}, event.Changes[1])

	require.Equal(t, PositionChange{
		Field: "Size",
		From:  "300",
		To:    "250",
	}, event.Changes[2])

	require.Equal(t, PositionChange{
		Field: "MarkPrice",
		From:  "0.03263",
		To:    "0.03280",
	}, event.Changes[3])

	require.Equal(t, PositionChange{
		Field: "UnrealisedPnl",
		From:  "-0.033",
		To:    "0.012",
	}, event.Changes[4])

	require.Equal(t, PositionChange{
		Field: "Leverage",
		From:  "1",
		To:    "2",
	}, event.Changes[5])
}

func TestMonitor_UpdatePosition_DetectsNoChanges(t *testing.T) {
	monitor := New()

	position := bybit.Position{
		PositionIdx:   0,
		Symbol:        "CHIPUSDT",
		Side:          "Buy",
		Size:          "300",
		TakeProfit:    "0.03891",
		StopLoss:      "0",
		MarkPrice:     "0.03263",
		UnrealisedPnl: "-0.033",
		Leverage:      "1",
	}

	monitor.UpdatePosition(position)

	event := monitor.UpdatePosition(position)

	require.Equal(t, PositionUpdated, event.Type)
	require.Empty(t, event.Changes)
}
