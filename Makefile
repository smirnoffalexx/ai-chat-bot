.PHONY: build run vet test tidy

build:
	go build -o bin/bot ./cmd/bot

run:
	go run ./cmd/bot

vet:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy
