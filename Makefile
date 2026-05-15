SHELL := /bin/bash

PEER_DIR := peer
BIN_DIR := bin
BINARY := $(BIN_DIR)/peer

.PHONY: test test-race vet lint build build-arm64 smoke ci clean

test:
	cd $(PEER_DIR) && go test ./...

test-race:
	cd $(PEER_DIR) && go test -race ./...

vet:
	cd $(PEER_DIR) && go vet ./...

lint:
	cd $(PEER_DIR) && test -z "$$(gofmt -l .)"

build:
	mkdir -p $(BIN_DIR)
	cd $(PEER_DIR) && go build -o ../$(BINARY) .

build-arm64:
	mkdir -p $(BIN_DIR)
	cd $(PEER_DIR) && GOOS=linux GOARCH=arm64 go build -o ../$(BINARY)-linux-arm64 .

smoke: build
	POKOINPOS_RUN_MODE=node \
	POKOINPOS_OPS_ADDR=:18080 \
	POKOINPOS_LISTEN_PORT=43100 \
	POKOINPOS_ADMIN_TOKEN=smoke-token \
	./$(BINARY) >/tmp/pokoinpos-smoke.log 2>&1 & \
	pid=$$!; \
	trap "kill $$pid >/dev/null 2>&1 || true" EXIT; \
	sleep 2; \
	curl -fsS http://127.0.0.1:18080/health >/dev/null; \
	curl -fsS http://127.0.0.1:18080/ready >/dev/null

ci: test test-race vet lint build-arm64 smoke

clean:
	rm -rf $(BIN_DIR)
