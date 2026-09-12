.PHONY: all build build-bot build-screener test vet clean deploy status

BINARY_BOT=bot
BINARY_SCREENER=screener
SERVICES=screener-long screener-short bot-long bot-short

all: build

build: build-bot build-screener

build-bot:
	@echo "[BUILD] Building bot..."
	go build -ldflags="-s -w" -o $(BINARY_BOT).tmp ./cmd/bot
	mv $(BINARY_BOT).tmp $(BINARY_BOT)

build-screener:
	@echo "[BUILD] Building screener..."
	go build -ldflags="-s -w" -o $(BINARY_SCREENER).tmp ./cmd/screener
	mv $(BINARY_SCREENER).tmp $(BINARY_SCREENER)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY_BOT) $(BINARY_SCREENER) $(BINARY_BOT).tmp $(BINARY_SCREENER).tmp

deploy:
	@echo "[DEPLOY] Pulling latest code..."
	git pull --ff-only origin sail-both-v1

	@echo "[DEPLOY] Running tests..."
	@go test ./...

	@echo "[DEPLOY] Building bot..."
	@go build -ldflags="-s -w" -o $(BINARY_BOT).tmp ./cmd/bot

	@echo "[DEPLOY] Building screener..."
	@go build -ldflags="-s -w" -o $(BINARY_SCREENER).tmp ./cmd/screener

	@echo "[DEPLOY] Build successful. Installing binaries..."
	@mv $(BINARY_BOT).tmp $(BINARY_BOT)
	@mv $(BINARY_SCREENER).tmp $(BINARY_SCREENER)

	@echo "[DEPLOY] Restarting services..."
	@systemctl restart $(SERVICES)

	@echo "[DEPLOY] Deployment successful!"

status:
	systemctl status $(SERVICES) --no-pager