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
	git pull origin sail-both-v1
	@make build
	@echo "[DEPLOY] Compilation successful. Restarting services..."
	sudo systemctl stop $(SERVICES)
	sudo systemctl start $(SERVICES)
	@echo "[DEPLOY] Deployment successful!"

status:
	sudo systemctl status $(SERVICES) --no-pager
