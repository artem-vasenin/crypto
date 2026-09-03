.PHONY: all build clean restart deploy status

BINARY_BOT=bot
BINARY_SCREENER=screener
SERVICES=screener-long screener-short bot-long bot-short

# Атомарная сборка во временные файлы
build:
	@echo "[BUILD] Compiling Go binaries..."
	go build -ldflags="-s -w" -o $(BINARY_BOT).tmp ./cmd/bot
	go build -ldflags="-s -w" -o $(BINARY_SCREENER).tmp ./cmd/screener

deploy:
	@echo "[DEPLOY] Pulling latest code..."
	git pull origin sail-both-v1
	@make build
	@echo "[DEPLOY] Compilation successful. Swapping binaries and restarting services..."
	sudo systemctl stop $(SERVICES)
	mv $(BINARY_BOT).tmp $(BINARY_BOT)
	mv $(BINARY_SCREENER).tmp $(BINARY_SCREENER)
	sudo systemctl start $(SERVICES)
	@echo "[DEPLOY] Deployment successful!"

status:
	sudo systemctl status $(SERVICES) --no-pager