.PHONY: lint test build clean ci

BIN := bin/approval-hub

lint:
	golangci-lint run ./...

test:
	go test ./... -race

build:
	mkdir -p bin
	go build -o $(BIN) ./cmd/approval-hub

clean:
	rm -rf bin

ci: lint test build
