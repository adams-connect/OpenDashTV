# OpenDashTV Makefile
# Optimized for Raspberry Pi 3B+ (ARMv7 32-bit / ARM64) and local development.

BINARY_NAME ?= dashboard-core
CMD_PKG     := ./cmd/dashboard
BIN_DIR     := bin

# Linker flags: strip debug symbols and symbol tables to reduce binary size by ~35%,
# keeping disk footprint low on MicroSD storage.
LDFLAGS := -s -w

.PHONY: all build build-pi build-pi64 test test-race lint clean

all: lint test build

## build: Compiles native binary for host OS/architecture
build:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PKG)

## build-pi: Cross-compiles for Raspberry Pi 3B+ (ARMv7 32-bit, default Raspberry Pi OS)
build-pi:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-armv7 $(CMD_PKG)

## build-pi64: Cross-compiles for 64-bit ARM (Raspberry Pi OS 64-bit / Pi 4/5)
build-pi64:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-arm64 $(CMD_PKG)

## test: Runs test suite
test:
	go test -v ./...

## test-race: Runs test suite with data race detector
test-race:
	go test -v -race ./...

## lint: Runs static analysis and vetting
lint:
	go vet ./...

## clean: Removes build outputs and test artifacts
clean:
	rm -rf $(BIN_DIR)
