```bash
go build -o bot ./cmd/bot 
```

```bash
go run cmd/screener/main.go --strategy long --interval 1m --config configs/config.json
```

```bash
go run cmd/screener/main.go --strategy short --interval 1m --config configs/config.json
```

```bash
./bot --strategy long --input long-screening.json --config configs/config.json
```

```bash
./bot --strategy short --input short-screening.json --config configs/config.json
```