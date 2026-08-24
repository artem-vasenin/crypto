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
│   │   ├── client_test.go
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

⸻

Current Goal

Build a read-only monitor for Bybit Futures positions that:

* receives current open Futures positions;
* monitors position changes through WebSocket;
* keeps an in-memory state of positions;
* detects opening, updating, partial closing and full closing of positions;
* later provides notifications;
* later provides AI-based analysis.

Automatic trading is not part of the current stage.

Grid-bot monitoring is also not currently implemented through the official documented API. The current monitor focuses on Futures positions.

⸻

Development Rules

* The project is being developed step by step.
* The developer is learning Go.
* Do not replace the project with a ready-made archive.
* Prefer small changes that can be tested immediately.
* Explain important Go concepts when introducing them.
* When changing code, always specify the file path before the code fragment.
* At the end of a completed development step, provide the full contents of every changed file.
* Do not create duplicate files or duplicate implementations when an existing component already handles the responsibility.
* Preserve working functionality unless there is a clear reason to change it.
* After a significant architectural change, update this file as a project checkpoint.
* Keep the application read-only.

⸻

Completed

Configuration

* Config loading from configs/config.json
* API key and secret loaded from .env
* API secrets excluded from JSON serialization
* Validation that API key and secret are present

⸻

Bybit REST API

* Bybit HMAC REST authentication
* GetPositions() implementation
* Request for linear USDT Futures positions
* Position model
* PositionResponse model
* REST API error handling

Current REST request:

GET /v5/position/list?category=linear&settleCoin=USDT

The monitor currently works with USDT linear Futures positions.

⸻

Flexible JSON Types

Bybit does not always return timestamp fields using the same JSON type.

For example:

"openTime": 1787461946292

or:

"updatedTime": "1787477526251"

The project therefore uses:

type FlexibleInt64 int64

with a custom UnmarshalJSON.

⸻

Bybit WebSocket

* Private WebSocket connection
* WebSocket authentication
* Subscription to position
* WebSocket heartbeat
* WebSocket message parsing
* Conversion from WebSocket position to common Position

Private WebSocket endpoint:

wss://stream.bybit.com/v5/private

Current subscription:

position

⸻

WebSocket Findings

Position WebSocket events contain the current state of the position.

The following real Bybit events have been tested:

* PnL changes
* TP changes
* SL changes
* position size changes
* partial position closing
* full position closing

Changing TP through the Bybit interface produces a position WebSocket event.

The event contains the complete current position state rather than only the changed field.

Example:

takeProfit
markPrice
unrealisedPnl
size
positionValue
...

Therefore the monitor keeps both the previous and current position when generating update events.

⸻

Monitor

Current State

Monitor currently stores:

type State struct {
    Positions []bybit.Position
}

The state is kept in memory.

⸻

Position Identity

A position is identified by:

Symbol + PositionIdx

Side is intentionally not part of the position identity.

This is important because in one-way mode the side can change while the position remains the same.

Example:

B3USDT + PositionIdx 0 + Buy
        ↓
B3USDT + PositionIdx 0 + Sell

This is treated as the same position.

positionIdx also allows future support for hedge mode:

Symbol + PositionIdx 1 → Buy
Symbol + PositionIdx 2 → Sell

⸻

Monitor Methods

Current methods:

func New() *Monitor
func (m *Monitor) UpdatePositions(
    positions []bybit.Position,
)
func (m *Monitor) Positions() []bybit.Position
func (m *Monitor) Position(
    symbol string,
    positionIdx int,
) (bybit.Position, bool)
func (m *Monitor) UpdatePosition(
    position bybit.Position,
) PositionEvent

⸻

Position Events

The Monitor now reports changes to the position lifecycle.

Current event types:

type PositionEventType string
const (
    PositionOpened  PositionEventType = "opened"
    PositionUpdated PositionEventType = "updated"
    PositionClosed  PositionEventType = "closed"
)

Current event structure:

type PositionEvent struct {
    Type     PositionEventType
    Previous *bybit.Position
    Current  *bybit.Position
}

⸻

Event Semantics

Opened

A position did not previously exist and a new non-zero position arrived.

Previous = nil
Current  = new position

⸻

Updated

A position already existed and a new non-zero state arrived.

Previous = old position
Current  = new position

This can represent:

* PnL change;
* TP change;
* SL change;
* mark price change;
* position size change;
* other position state changes.

The Monitor currently does not distinguish these individual changes.

⸻

Closed

An existing position receives:

size == "0"

The position is removed from Monitor state.

The event contains:

Previous = last known position
Current  = nil

⸻

Unknown Close

If Monitor receives:

size == "0"

for a position that does not exist in its current state, no position is removed and an empty PositionEvent is returned.

This protects the state from unexpected or duplicate close messages.

⸻

Main Application Flow

Current architecture:

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
        ↓
   ToPosition()
        ↓
Monitor.UpdatePosition()
        ↓
PositionEvent
        ↓
    main.go

⸻

REST Initialization

main.go currently:

1. loads configuration;
2. creates Bybit REST client;
3. retrieves current positions;
4. creates Monitor;
5. initializes Monitor state using UpdatePositions();
6. prints current positions.

REST initialization does not generate position events.

This is intentional.

REST provides the initial state:

REST
 ↓
initial state
 ↓
Monitor

WebSocket provides subsequent changes:

WebSocket
 ↓
UpdatePosition()
 ↓
PositionEvent

⸻

Real WebSocket Testing

Two real test positions are currently available for development:

CHIPUSDT
VIRTUALUSDT

Both currently use:

PositionIdx: 0
Side: Buy
Leverage: 1x

Approximate position sizes during testing:

CHIPUSDT    300
VIRTUALUSDT 13

Small positions are intentionally used to safely observe Bybit API and WebSocket behaviour.

⸻

Real Event Test

Changing TP on CHIPUSDT produced:

Type: updated
Symbol: CHIPUSDT
Side: Buy

Changing TP on VIRTUALUSDT produced:

Type: updated
Symbol: VIRTUALUSDT
Side: Buy

This confirms that:

Bybit WebSocket
        ↓
position event
        ↓
ToPosition()
        ↓
Monitor.UpdatePosition()
        ↓
PositionEvent

works with real Bybit data.

⸻

Current Tests

Bybit tests

Current tests cover:

* signature generation
* signature payload generation

Run with:

go test ./...

or:

go test -race ./... -v

⸻

Monitor tests

Current tests cover:

* adding a new position;
* updating an existing position;
* removing a closed position;
* avoiding duplicates;
* storing multiple positions;
* updating only the matching position;
* changing position side;
* retrieving a position with Position();
* returning not-found from Position();
* distinguishing PositionIdx;
* generating PositionOpened;
* generating PositionUpdated;
* generating PositionClosed.

Latest test command:

go test -race ./... -v

Result:

PASS

No race conditions were detected.

⸻

Known WebSocket Issue

During previous testing the WebSocket occasionally produced:

WebSocket read error: local error: tls: bad record MAC

At one point this also caused:

fatal error: all goroutines are asleep - deadlock!

The connection later remained stable for approximately 10 minutes during testing.

The issue has not yet been fully solved.

It remains a known reliability problem and should be addressed before the monitor is considered production-ready.

⸻

Important Current Limitation

Monitor currently uses:

[]bybit.Position

rather than a map.

This is acceptable at the current scale and keeps the implementation simple while the event model is being developed.

A future refactoring may use:

map[PositionKey]bybit.Position

with a key based on:

Symbol + PositionIdx

but this has intentionally not been implemented yet.

Do not refactor to a map until there is a concrete reason to do so.

⸻

Current Architecture Status

                    Bybit
                      │
          ┌───────────┴───────────┐
          │                       │
        REST                  WebSocket
          │                       │
          ▼                       ▼
    []Position                wsPosition
          │                       │
          │                    ToPosition()
          │                       │
          ▼                       ▼
       Monitor ◄──────────── UpdatePosition()
          │                       │
          │                       ▼
          │                 PositionEvent
          │
          ▼
   Current positions

⸻

Next Development Step

The next planned feature is field-level position change analysis.

The current event tells us:

PositionUpdated

but does not yet tell us what changed.

The next layer should compare:

Previous
    ↓
    compare
    ↓
Current

and detect changes such as:

TP changed
SL changed
PnL changed
Size changed
MarkPrice changed
Leverage changed

The important architectural principle is:

PositionEvent
    │
    ├── Previous
    └── Current
             │
             ▼
      change analysis

PositionEvent itself should remain generic.

The bybit package should remain responsible for:

* REST API;
* WebSocket;
* JSON parsing;
* Bybit models.

Application-specific interpretation of position changes should remain in internal/monitor.

⸻

Future Development

Planned later stages:

1. Field-level position change analysis
2. Distinguish partial close from ordinary size changes
3. Better event output/logging
4. Notification layer
5. Improve WebSocket reconnect/reliability
6. Separate application responsibilities from main.go
7. Consider replacing []Position with a map if justified
8. AI-based position analysis
9. Grid-bot monitoring if a suitable supported API becomes available
10. Automatic trading only as a completely separate future stage

⸻

Current Checkpoint

At this checkpoint the project successfully:

* loads configuration;
* authenticates with Bybit REST;
* retrieves current USDT Futures positions;
* connects to private Bybit WebSocket;
* authenticates WebSocket;
* subscribes to position;
* parses position events;
* converts WebSocket positions to the common Position model;
* stores current positions in Monitor;
* identifies positions using Symbol + PositionIdx;
* retrieves individual positions from Monitor;
* detects opened positions;
* detects updated positions;
* detects closed positions;
* keeps previous/current state in PositionEvent;
* sends Monitor events to main.go;
* verified updated events using real CHIPUSDT data;
* verified updated events using real VIRTUALUSDT data;
* all tests pass;
* go test -race ./... -v passes.

Checkpoint status: STABLE

Next task:

Implement field-level analysis of Previous → Current.