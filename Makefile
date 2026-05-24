BIN := inv-driver
PKG := ./cmd/inv-driver
BUILD_DIR := build
HOST_BIN := $(BUILD_DIR)/$(BIN)
ARMV7_BIN := $(BUILD_DIR)/$(BIN)-armv7

LEGACY_BRIDGE_BIN := $(BUILD_DIR)/legacy-bridge
LEGACY_BRIDGE_ARMV7 := $(BUILD_DIR)/legacy-bridge-armv7
LEGACY_BRIDGE_PKG := ./cmd/legacy-bridge

ECU_WEB_BIN := $(BUILD_DIR)/ecu-web
ECU_WEB_ARMV7 := $(BUILD_DIR)/ecu-web-armv7
ECU_WEB_PKG := ./cmd/ecu-web
ECU_WEB_DIR_SRC := cmd/ecu-web/web

ECU_HOST ?= root@10.25.1.33
ECU_DIR ?= /home/applications/inv-driver
LEGACY_BRIDGE_ECU_DIR ?= /home/applications/legacy-bridge
ECU_WEB_ECU_DIR ?= /home/applications/ecu-web

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

PROTOC ?= protoc

# proto target is a manual prerequisite. It is idempotent (overwrites
# the same wire/busmgr.pb.go) but we deliberately don't chain it into
# build/build-arm so cross-platform builds don't require protoc on
# every host. Run `make proto` after editing proto/busmgr.proto.
.PHONY: all build build-arm clean fmt vet test deploy proto \
        build-legacy-bridge build-legacy-bridge-arm deploy-legacy-bridge \
        web web-test build-ecu-web build-ecu-web-arm deploy-ecu-web

all: build-arm build-legacy-bridge-arm build-ecu-web-arm

proto:
	$(PROTOC) -I proto --go_out=wire --go_opt=paths=source_relative proto/busmgr.proto

build:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(HOST_BIN) $(PKG)

build-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build -ldflags '-s -w' -trimpath -o $(ARMV7_BIN) $(PKG)
	@echo "built $(ARMV7_BIN) ($$(wc -c <$(ARMV7_BIN)) bytes)"

build-legacy-bridge:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build \
		-ldflags '-X main.version=$(VERSION)' \
		-o $(LEGACY_BRIDGE_BIN) $(LEGACY_BRIDGE_PKG)

build-legacy-bridge-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build -ldflags '-s -w -X main.version=$(VERSION)' -trimpath \
		-o $(LEGACY_BRIDGE_ARMV7) $(LEGACY_BRIDGE_PKG)
	@echo "built $(LEGACY_BRIDGE_ARMV7) ($$(wc -c <$(LEGACY_BRIDGE_ARMV7)) bytes)"

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

deploy: build-arm
	ssh $(ECU_HOST) 'mkdir -p $(ECU_DIR)'
	ssh $(ECU_HOST) 'cat > $(ECU_DIR)/$(BIN).new' < $(ARMV7_BIN)
	ssh $(ECU_HOST) 'chmod +x $(ECU_DIR)/$(BIN).new && mv $(ECU_DIR)/$(BIN).new $(ECU_DIR)/$(BIN)'
	@echo "deployed to $(ECU_HOST):$(ECU_DIR)/$(BIN)"

deploy-legacy-bridge: build-legacy-bridge-arm
	ssh $(ECU_HOST) 'mkdir -p $(LEGACY_BRIDGE_ECU_DIR)'
	ssh $(ECU_HOST) 'cat > $(LEGACY_BRIDGE_ECU_DIR)/legacy-bridge.new' < $(LEGACY_BRIDGE_ARMV7)
	ssh $(ECU_HOST) 'chmod +x $(LEGACY_BRIDGE_ECU_DIR)/legacy-bridge.new && mv $(LEGACY_BRIDGE_ECU_DIR)/legacy-bridge.new $(LEGACY_BRIDGE_ECU_DIR)/legacy-bridge'
	@echo "deployed to $(ECU_HOST):$(LEGACY_BRIDGE_ECU_DIR)/legacy-bridge"

# web rebuilds the embedded SPA bundle (cmd/ecu-web/web/dist) with Bun.
# Like proto, it is a manual prerequisite: dist/ is committed so cross
# builds don't require Bun, but run `make web` after editing the
# frontend. `bun run build` runs the 7-day dependency cooldown gate first.
web:
	cd $(ECU_WEB_DIR_SRC) && bun install && bun run build

# web-test runs the cooldown gate + Lit component tests.
web-test:
	cd $(ECU_WEB_DIR_SRC) && bun run cooldown && bun test

build-ecu-web:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build \
		-ldflags '-X main.version=$(VERSION)' \
		-o $(ECU_WEB_BIN) $(ECU_WEB_PKG)

build-ecu-web-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build -ldflags '-s -w -X main.version=$(VERSION)' -trimpath \
		-o $(ECU_WEB_ARMV7) $(ECU_WEB_PKG)
	@echo "built $(ECU_WEB_ARMV7) ($$(wc -c <$(ECU_WEB_ARMV7)) bytes)"

deploy-ecu-web: build-ecu-web-arm
	ssh $(ECU_HOST) 'mkdir -p $(ECU_WEB_ECU_DIR)'
	ssh $(ECU_HOST) 'cat > $(ECU_WEB_ECU_DIR)/ecu-web.new' < $(ECU_WEB_ARMV7)
	ssh $(ECU_HOST) 'chmod +x $(ECU_WEB_ECU_DIR)/ecu-web.new && mv $(ECU_WEB_ECU_DIR)/ecu-web.new $(ECU_WEB_ECU_DIR)/ecu-web'
	@echo "deployed to $(ECU_HOST):$(ECU_WEB_ECU_DIR)/ecu-web"

clean:
	rm -rf $(BUILD_DIR)
