.PHONY: all build clean restart deploy status

BINARY_BOT=bot
BINARY_SCREENER=screener
SERVICES=screener-long screener-short bot-long bot-short

# Компиляция бинарников с флагами оптимизации (минус отладочная информация)
build:
	@echo "[BUILD] Compiling Go binaries..."
	go build -ldflags="-s -w" -o $(BINARY_BOT) ./cmd/bot
	go build -ldflags="-s -w" -o $(BINARY_SCREENER) ./cmd/screener

# Остановка сервисов, Pull, Сборка, Запуск
deploy:
	@echo "[DEPLOY] Pulling latest code..."
	git pull origin sail-both-v1
	@echo "[DEPLOY] Stopping systemd services..."
	sudo systemctl stop $(SERVICES)
	@make build
	@echo "[DEPLOY] Starting systemd services..."
	sudo systemctl start $(SERVICES)
	@echo "[DEPLOY] Deployment successful!"

status:
	sudo systemctl status $(SERVICES) --no-pager