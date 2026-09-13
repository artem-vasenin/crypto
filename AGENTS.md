AGENTS.md

Project: Universal Bybit Crypto Screener & Trading Bot

This is a Go project for researching and trading USDT perpetual contracts on Bybit.

The project is research-first. It is not a generic trading framework and should not be changed into one without an explicit reason.

The system consists of:

* market data collection;
* technical analysis;
* market structure analysis;
* candidate screening;
* directional strategy evaluation;
* risk management;
* order execution;
* position management;
* trade snapshots for later AI/research analysis.

The project currently supports these strategy names:

* long
* short
* long-grid
* short-grid
* neutral-grid

Grid strategies are primarily screening/research strategies. The trading bot executes directional long and short strategies.

⸻

1. Important working rules

Language and version

* Go 1.26.5+ is the intended project version.
* Do not downgrade the project’s Go version just to make code work.
* Production code must be compatible with the Go version declared in go.mod.

Coding style

Prefer:

* simple Go;
* explicit code;
* small functions;
* clear names;
* standard library where practical;
* minimal abstractions;
* no unnecessary frameworks;
* no speculative refactoring.

Do not introduce architectural changes merely because another design looks theoretically cleaner.

Before changing behavior, understand why the existing behavior exists.

⸻

2. Source of truth

When documentation and source code disagree:

1. Current source code is the source of truth.
2. Tests are the next strongest source of truth.
3. README.md, STRUCTURE.md, and AUDIT.md describe intended behavior and historical decisions.

Do not assume that documentation is newer than the code.

After a significant architectural change, update the relevant documentation.

⸻

3. Project documentation

The root documentation has different purposes.

README.md

User-facing project documentation:

* purpose;
* setup;
* configuration;
* commands;
* running screener/bot;
* deployment/use instructions.

STRUCTURE.md

Architecture documentation:

* packages;
* data flow;
* analysis pipeline;
* strategy pipeline;
* execution pipeline;
* important models and relationships.

AUDIT.md

Research/audit history:

* identified problems;
* why they are problems;
* changes made;
* unresolved concerns;
* things that still need validation.

Do not silently rewrite historical audit conclusions just because code has changed.

⸻

4. Main architectural idea

The system deliberately does not use one universal score for every decision.

Directional strategies are evaluated through separate quality dimensions:

1. TrendQuality
2. AssetQuality
3. EntryQuality
4. MarketQuality
5. DerivativesQuality

These dimensions should remain conceptually separate.

A high asset-quality score must not automatically compensate for a fundamentally bad entry.

Likewise, strong market liquidity must not turn a bad directional setup into a good trade.

⸻

5. Directional analysis

The directional pipeline currently evaluates:

TrendQuality

Primarily:

* 1h structure;
* 4h structure;
* 30m structure.

AssetQuality

Primarily:

* 24h performance;
* 3d performance;
* 7d performance;
* RSI 1h;
* RSI 4h.

EntryQuality

Primarily:

* 5m structure;
* 15m structure;
* RSI;
* range position;
* proximity to important levels.

MarketQuality

Primarily:

* spread;
* turnover;
* volume;
* volume behavior.

DerivativesQuality

Primarily:

* funding;
* open interest;
* order book.

⸻

6. Market structure

Structure states include:

* HH
* HL
* LH
* LL

Important interpretation:

Bullish structure

HH + HL

Bearish structure

LH + LL

Conflict / expansion structure

HH + LL
or
LH + HL

A conflict structure is not a half-strength bullish/bearish structure.

It indicates that highs and lows are expanding in different directions and should not receive the normal directional score.

Current directional structure scoring therefore treats conflict as:

conflict → 0

rather than:

conflict → 50

This distinction is important.

⸻

7. Higher-timeframe conflict

1h and 4h structure are particularly important.

A directional trade should not blindly pass when the higher timeframes are in conflict.

The system has explicit hard blocks for:

* long against clear bearish HTF structure;
* short against clear bullish HTF structure;
* long/short when 1h or 4h has a conflict structure.

The exact implementation is in:

internal/strategies/

Do not remove these blocks merely to increase the number of candidates.

⸻

8. MarketContext

Candidates contain a MarketContext.

It describes information that should not be hidden inside a generic score.

Current concepts include:

* market direction;
* market regime;
* higher-timeframe conflict;
* local resistance;
* distance to local resistance.

Typical regime values include:

* trend
* expansion
* uncertain

This context exists because the same numerical indicators can mean very different things in different market regimes.

⸻

9. Late-entry protection

A particular research problem discovered in the project is entering a long after a strong upward move when price is already close to local resistance.

The current long strategy contains a specific late-entry protection gate.

It considers conditions such as:

* local resistance exists;
* price is very close to it;
* RSI 15m is elevated;
* 24h price change is already strongly positive.

This is intentionally more specific than simply saying:

RSI is high, therefore don’t buy.

Do not replace this with simplistic RSI-only logic without testing.

⸻

10. Indicators

The project currently uses indicators across several timeframes.

Important fields include:

* RSI 5m;
* RSI 15m;
* RSI 1h;
* RSI 4h;
* ATR 5m;
* ATR 15m;
* ATR 1h;
* ATR 4h;
* ATR percentages;
* volume ratio;
* volume trend;
* price relative to EMA10 on 15m;
* BTC 15m trend context.

Some fields may intentionally be unavailable.

For example:

"btc_15m_trend_pct": null

is different from:

"btc_15m_trend_pct": 0

null means the value was not available.

Do not invent missing market data by replacing unavailable values with zero.

⸻

11. EMA

PriceVsEMA10_15m is derived from the 15m candles and the EMA10.

It is contextual information, not a standalone buy/sell signal.

Do not turn every indicator into an independent trading rule without research.

⸻

12. Levels

Support/resistance values can be unavailable.

For example:

"resistance": null

does not mean:

there is no resistance

It means the current level calculation did not find a suitable level under the current rules.

This distinction is important when interpreting snapshots.

The project should prefer explicit unknown/unavailable semantics over fake zero values.

⸻

13. ATR and volatility

ATR is used both as an indicator and as a risk/context measure.

High volatility can make a technically attractive setup unsuitable for the strategy.

Current directional logic contains protection against extreme ATR conditions.

Do not assume:

higher volatility = better opportunity.

For this project, excessive volatility can be a reason to reject a setup.

⸻

14. Screener eligibility

Current directional gates are approximately:

TrendQuality        >= 60
AssetQuality        >= 55
EntryQuality        >= 60
MarketQuality       >= 60
DerivativesQuality  >= 40

These thresholds are part of the current research baseline.

They should be changed only deliberately and preferably after examining real snapshots.

The bot checks:

Decision.Eligible

It does not depend on a universal MinScore.

⸻

15. Ranking

The screener should prioritize:

1. eligible candidates;
2. EntryQuality;
3. AssetQuality.

This is intentional.

The project is trying to avoid the situation where a coin with excellent historical/asset characteristics outranks a coin with a materially better current entry.

⸻

16. Snapshot philosophy

Trade snapshots are a major research feature.

A snapshot represents the information available around an actual trade execution.

The purpose is to answer questions such as:

* Why did the system enter?
* What did the market look like at entry?
* Were higher timeframes aligned?
* Was the entry late?
* Was price close to resistance/support?
* What did funding/OI/order book show?
* Which fields are actually useful for future AI analysis?

Snapshots are research data.

Do not modify historical snapshots to make them agree with new strategy logic.

⸻

17. Snapshot field discipline

When analyzing a snapshot:

* distinguish measured data from derived data;
* distinguish unavailable data from zero;
* do not invent fields;
* do not infer causation from correlation without evidence;
* do not claim a trade worked or failed solely from entry conditions;
* separate asset quality from entry quality;
* pay attention to timeframe conflicts.

Example:

price ↑ + OI ↓

does not automatically prove a specific cause.

It can be consistent with several market mechanisms and should be described carefully.

⸻

18. Execution and risk management

The bot uses:

* leverage;
* fixed margin limits;
* maximum total margin;
* maximum active positions;
* stop loss;
* take profit;
* trailing stop;
* maker/taker cost assumptions.

The bot must not silently increase position quantity simply to satisfy an exchange minimum.

If the calculated quantity is below an exchange minimum, the safer behavior is to reject/skip the trade rather than silently increasing exposure.

⸻

19. Trailing stop behavior

An important bug was discovered and fixed.

Previously, the trailing stop could tighten almost immediately after entry.

This could destroy the intended initial structural stop.

Current behavior:

initial structural SL
↓
position moves favorably by 1R
↓
trailing becomes active
↓
SL can start following price

Where:

1R = initial distance between entry and initial stop

For a long:

activation price = entry + initial risk

For a short:

activation price = entry - initial risk

Do not remove the 1R activation requirement without deliberate testing.

⸻

20. Take profit / stop loss

TP and SL are supplied with the main order where appropriate.

Do not blindly re-send trading-stop configuration after every fill if the current execution architecture already places the protection correctly.

Any change to order lifecycle must consider:

* order ID;
* side;
* fill state;
* exchange behavior;
* WebSocket execution events;
* foreign positions.

⸻

21. Managed vs foreign positions

The bot may see positions that it did not open.

Such positions must be distinguishable from bot-managed positions.

A foreign position should not automatically be modified by the bot’s position-management logic.

When debugging execution, always check whether:

managed = true

or:

managed = false

before concluding that the bot changed a position.

⸻

22. Bybit data

The project works with Bybit USDT perpetual / linear market data.

Relevant data includes:

* ticker;
* klines;
* open interest;
* funding;
* order book;
* positions;
* orders;
* execution events.

Do not assume that a REST value and a WebSocket value have identical timing.

Timestamp/look-ahead issues are important for research correctness.

⸻

23. BTC context

BTC 15m trend is used as contextual information.

The implementation uses 15-minute UTC bucket boundaries.

This is better than resetting every 15 minutes from process startup, but it is still not identical to using the exact historical 15m candle open.

Therefore:

BTC 15m trend

should currently be treated as useful context, not as a perfectly candle-aligned historical feature.

Do not present it as look-ahead-safe historical candle data without verifying the implementation.

⸻

24. Tests

Important tests cover at least:

* strategy structure scoring;
* conflict structures;
* late long rejection;
* EMA calculation;
* trailing activation at 1R;
* existing risk/execution behavior.

When changing trading logic:

1. update/add a focused test;
2. run the test suite;
3. inspect failures;
4. only then consider broader refactoring.

Never delete a failing test simply because it prevents a new implementation from passing.

⸻

25. Verification honesty

Never say that code was tested, built, vetted, or verified unless it was actually executed.

Be explicit about the environment used for verification.

A previous source verification was performed using Go 1.23.2 with temporary compatibility adjustments because the available runtime was older than the project’s intended Go 1.26.5.

Therefore, that verification does not prove Go 1.26.5 runtime compatibility.

⸻

26. Deployment

Production deployment is separate from the trading logic.

Current production deployment uses CI/CD and a VPS.

Do not overwrite or reconstruct deployment configuration from memory.

In particular:

Makefile
.github/workflows/deploy.yml

must be treated as authoritative files from the current source tree.

If they are missing from an old archive, do not invent their contents.

⸻

27. How to approach changes

When asked to modify the project:

First

Understand the existing implementation.

Second

Identify the exact files/functions affected.

Third

Explain the proposed behavioral change in simple terms.

Fourth

Make the smallest reasonable change.

Fifth

Add or update tests where behavior changed.

Sixth

Run verification when possible.

Seventh

Update documentation if the architectural behavior changed.

Do not perform unrelated cleanup in the same change.

⸻

28. Research philosophy

This project is not trying to find a magical indicator combination.

The main research questions are:

* What market conditions produce reliable entries?
* Which fields actually contain useful information?
* Which fields are redundant?
* Which conditions create false positives?
* Which signals are only attractive because of hindsight?
* How does entry quality differ from asset quality?
* How does volatility affect the usefulness of a setup?
* How close to support/resistance is too close?
* What happens when timeframes disagree?

Real snapshots should be used to answer these questions.

Do not optimize thresholds against a tiny number of examples and then declare the strategy validated.

⸻

29. Current known research concerns

Important areas already identified include:

* late long entries after strong upward movement;
* higher-timeframe conflict;
* correct treatment of HH+LL / LH+HL;
* immediate trailing activation;
* usefulness of EMA10;
* quality of local support/resistance;
* BTC 15m context alignment;
* distinction between unavailable and zero values;
* avoiding look-ahead bias;
* whether current scoring thresholds generalize beyond a few snapshots.

These are research questions, not automatically solved problems.

⸻

30. Current reference snapshot set

The project has been tested/reviewed against actual trade snapshots including:

* WLFIUSDT long;
* PUMPFUNUSDT short;
* WIFUSDT short;
* ATOMUSDT short;
* AVAXUSDT short.

The WLFI long was especially useful for discovering two issues:

1. higher-timeframe conflict was being treated too generously;
2. trailing stop activation was happening too early.

The shorts were manually closed during deployment/CI work, so their manual closure must not be interpreted as evidence of strategy success or failure.

Do not use this small sample to claim statistical performance.

⸻

31. Important interpretation rule

When reviewing a trade, separate:

What the system knew at entry

from:

What happened after entry

Never use post-entry information to justify an entry decision.

This distinction is critical for avoiding look-ahead bias.

⸻

32. If context is lost

If this project is opened in a new AI conversation, read these files first:

AGENTS.md
README.md
STRUCTURE.md
AUDIT.md

Then inspect the actual source tree.

The source archive/version supplied with the conversation is the final authority for exact implementation.

A new AI should not assume that an older conversation’s implementation is still current.

⸻

33. Expected collaboration style

The user is learning Go and wants to understand the project rather than blindly receive generated code.

When explaining changes:

* start with the simple idea;
* explain why the change is needed;
* identify the exact file;
* show the relevant code;
* after a major completed change, provide the full contents of changed files;
* do not hide important logic behind vague descriptions;
* do not claim code was tested unless it was actually tested;
* if something is uncertain, say so explicitly.

The goal is to build a system the user understands and can maintain themselves.