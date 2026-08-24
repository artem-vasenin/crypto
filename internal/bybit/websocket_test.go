package bybit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWebSocketMessage_Position(t *testing.T) {
	message := []byte(`{
		"topic":"position",
		"creationTime":1787588403634,
		"data":[
			{
				"positionIdx":0,
				"symbol":"XRPPERP",
				"side":"Buy",
				"size":"6.6",
				"entryPrice":"1.5128",
				"leverage":"1",
				"markPrice":"1.5117",
				"takeProfit":"1.6986",
				"stopLoss":"1.1042",
				"unrealisedPnl":"0.08052",
				"openTime":"1787586078635",
				"updatedTime":1787588403631
			}
		]
	}`)

	result, err := ParseWebSocketMessage(message)

	require.NoError(t, err)

	require.Equal(
		t,
		"position",
		result.Topic,
	)

	require.Len(
		t,
		result.Data,
		1,
	)

	position := result.Data[0]

	require.Equal(
		t,
		"XRPPERP",
		position.Symbol,
	)

	require.Equal(
		t,
		"Buy",
		position.Side,
	)

	require.Equal(
		t,
		"1.6986",
		position.TakeProfit,
	)

	require.Equal(
		t,
		FlexibleInt64(1787588403631),
		position.UpdatedTime,
	)
}

func TestParseWebSocketMessage_NonPositionTopic(
	t *testing.T,
) {
	message := []byte(`{
		"topic":"wallet",
		"op":"subscribe"
	}`)

	result, err := ParseWebSocketMessage(message)

	require.NoError(t, err)

	require.Equal(
		t,
		"wallet",
		result.Topic,
	)

	require.Empty(
		t,
		result.Data,
	)
}
