# Набор коротких команд для локальной разработки и проверки проекта.

.PHONY: test build run fmt vet clean

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

test: fmt
	go test ./...

build: test
	mkdir -p bin
	go build -o bin/crypto-coin-analyzer ./cmd/analyzer

run:
	go run ./cmd/analyzer -symbol SOLUSDT -out SOLUSDT.json

clean:
	rm -rf bin *.json
