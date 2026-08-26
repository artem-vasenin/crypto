# Data model notes

The output deliberately keeps the old blocks (`market`, `indicators`, `structure`, `levels`, `derivatives`, `strategies`) and adds new blocks instead of replacing the old format.

## New blocks

- `trend`: EMA20/50/200 and distance from current price.
- `momentum`: 1h/4h/12h/24h price changes.
- `volume`: 5m/15m/1h activity and trade-side volume.
- `volatility`: ATR normalized to price and realized volatility.
- `ranges`: current position inside 24h/3d/7d ranges.
- `order_book`: depth within configurable percentages around price and bid/ask imbalance.
- `trades`: recent public trade buy/sell pressure.
- `btc_context`: BTC direction and correlation with the candidate.
- expanded `derivatives`: OI and funding history statistics.

## Why no whale levels

A public order book shows visible resting orders, not the identity or true intent of the owner. The project therefore calls them liquidity/walls rather than whales.

## Why no news

The screener is intentionally limited to market data from Bybit. News/fundamental enrichment can be added later as a separate stage without contaminating the market-data collector.
