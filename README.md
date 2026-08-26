# Bybit Screener v2

Go 1.26 project for collecting public Bybit linear-USDT market data and generating a structured JSON snapshot for AI analysis.

Screener does not open or manage trading positions.

Its task is to:

1. filter the Bybit market;
2. collect detailed market data for selected coins;
3. calculate technical and derivatives indicators;
4. evaluate several trading strategies;
5. select the best candidates for the requested strategy;
6. save the result as JSON for further AI analysis.

---

## Supported strategies

The screener supports four strategies:

- `short-grid`
- `long-grid`
- `short`
- `long`

The strategy is selected using the `-strategy` command-line flag.

### Short Grid

```bash
go run ./cmd/screener -strategy short-grid
```

```bash
go run ./cmd/screener -strategy short
```

```bash
go run ./cmd/screener -strategy long-grid
```

```bash
go run ./cmd/screener -strategy long
```