.PHONY: build test clean run-relay run-listen

BINARY=rift
BUILD_DIR=bin

build:
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY) ./cmd/rift

test:
	go test ./... -v

clean:
	rm -rf $(BUILD_DIR)

run-relay:
	go run ./cmd/rift relay

run-listen:
	go run ./cmd/rift listen --to localhost:8080

# Cross-compile
build-all:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-linux-amd64   ./cmd/rift
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY)-linux-arm64   ./cmd/rift
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-darwin-amd64  ./cmd/rift
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY)-darwin-arm64  ./cmd/rift
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/rift
