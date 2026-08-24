# Bybit Monitor — Project State

## Project

Read-only monitor for Bybit Futures positions.

Go version: `1.26.5`

Module: `bybit_monitor`

---

## Project Structure

```text
bybit-monitor/
├── cmd/
│   └── monitor/
│       └── main.go
├── internal/
│   ├── bybit/
│   │   ├── client.go
│   │   ├── websocket.go
│   │   ├── models.go
│   │   └── types.go
│   ├── monitor/
│   │   ├── monitor.go
│   │   └── monitor_test.go
│   └── config/
│       └── config.go
├── configs/
│   └── config.json
├── go.mod
└── README.md
```

---

## Current Goal

Build a **read-only Bybit monitor** that:

- receives current open Futures positions;
- monitors position changes through WebSocket;
- keeps an in-memory state of positions;
- detects opening, updating, partial closing and full closing of positions;
- will later be extended with notifications and AI analysis.

Automatic trading is **not part of the current stage**. It may be developed later as a separate stage.

Grid-bot monitoring is also not currently implemented through the official documented API. The current monitor focuses on Futures positions.

---

## Done

- [x] Config loading from `configs/config.json`
- [x] API key and secret loaded from `.env`
- [x] API secrets excluded from JSON serialization
- [x] Bybit HMAC REST authentication
- [x] `GetPositions()` implementation
- [x] Bybit `Position` model
- [x] `PositionResponse` model
- [x] `FlexibleInt64` for timestamps that may arrive as numbers or strings
- [x] Private Bybit WebSocket connection
- [x] WebSocket authentication
- [x] WebSocket subscription to `position`
- [x] WebSocket heartbeat
- [x] WebSocket message parsing
- [x] Conversion from WebSocket position to common `Position`
- [x] Monitor state
- [x] Adding new positions
- [x] Updating existing positions
- [x] Removing fully closed positions
- [x] Tests for monitor state updates
- [x] Tested TP changes through WebSocket
- [x] Tested SL changes through WebSocket
- [x] Tested partial position closing
- [x] Tested full position closing

---

## Current Architecture

```text
config.json + .env
        ↓
internal/config
        ↓
      Config
        ↓
internal/bybit/client.go
        ↓
     REST API
        ↓
   []Position
        ↓
internal/monitor
        ↑
        │
internal/bybit/websocket.go
        ↑
   Bybit WebSocket
        ↓
 position events
```

`main.go` currently wires these components together and also contains most of the application loop. Later we will gradually move responsibilities out of `main.go`.

---

## Important Bybit Findings

### REST positions

Current REST request:

```text
GET /v5/position/list?category=linear&settleCoin=USDT
```

The monitor currently works with USDT linear Futures positions.

### WebSocket

Private WebSocket endpoint:

```text
wss://stream.bybit.com/v5/private
```

Current subscription:

```text
position
```

### Position opening/updating

Position WebSocket events contain the current state of the position.

We tested:

- PnL changes;
- TP changes;
- SL changes;
- position size changes.

### Partial close

A partial close produces a position event with a smaller `size`.

Example:

```text
21600 → 21000
```

The monitor updates the existing position.

### Full close

When a position is completely closed, Bybit sends a position event similar to:

```json
{
  "side": "",
  "size": "0"
}
```

The monitor treats `size == "0"` as a signal to remove the position from its state.

---

## Important Technical Details

### FlexibleInt64

Bybit does not always return timestamp fields using the same JSON type.

For example, a field may arrive as:

```json
"openTime": 1787461946292
```

or:

```json
"updatedTime": "1787477526251"
```

Therefore the project uses:

```go
type FlexibleInt64 int64
```

with a custom `UnmarshalJSON`.

---

## Current Monitor Logic

`Monitor.UpdatePosition()` currently identifies a position using:

```text
Symbol + Side
```

and handles:

```text
new position
    → append

existing position
    → replace

size == "0"
    → remove
```

### Important TODO

Before expanding the monitor significantly, review whether `Symbol + Side` is sufficient as a unique position identifier.

Bybit provides `positionIdx`, and this should be considered before supporting more complex account/position modes.

---

## Known WebSocket Issue

During testing, the WebSocket occasionally produced:

```text
WebSocket read error: local error: tls: bad record MAC
```

At one point this also caused:

```text
fatal error: all goroutines are asleep - deadlock!
```

The connection later remained stable for around 10 minutes during testing, so this issue has not yet been fully solved.

It is currently a known reliability issue and should be addressed before the monitor is considered production-ready.

---

## Test Position Used During Development

For experiments we used:

```text
Symbol: B3USDT
Side: Sell
Leverage: 1x
Position value: approximately 10 USDT
```

The position was intentionally used as a small test position to observe Bybit API and WebSocket behaviour.

The position has subsequently been fully closed.

---

## Current Tests

Monitor tests currently cover:

- adding a new position;
- updating an existing position;
- removing a closed position;
- avoiding duplicates;
- storing multiple positions;
- updating only the matching position.

All current tests passed during development.

Run tests with:

```bash
go test ./...
```

Run the monitor with:

```bash
go run ./cmd/monitor/
```

---

## Current Next Step

Before adding new functionality:

1. Review position identity and `positionIdx`.
2. Improve the separation of responsibilities between `main.go`, `bybit`, and `monitor`.
3. Keep the application read-only.
4. Continue testing against real Bybit WebSocket events.
5. Later add notifications.
6. Later consider AI-based analysis.
7. Automatic trading is a future, separate stage.

---

## Development Rules

- The project is being developed step by step.
- Do not replace the whole project with a ready-made archive.
- Explain Go concepts in detail because the developer is learning Go.
- Prefer small changes that can be tested immediately.
- After showing code fragments, provide the complete code of every changed file.
- Do not create duplicate files or duplicate implementations when an existing component already handles the responsibility.
- Preserve working functionality unless there is a clear reason to change it.
