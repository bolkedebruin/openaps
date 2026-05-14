BIN := inv-driver
PKG := ./cmd/inv-driver
BUILD_DIR := build
HOST_BIN := $(BUILD_DIR)/$(BIN)
ARMV7_BIN := $(BUILD_DIR)/$(BIN)-armv7

LEGACY_BRIDGE_BIN := $(BUILD_DIR)/legacy-bridge
LEGACY_BRIDGE_ARMV7 := $(BUILD_DIR)/legacy-bridge-armv7
LEGACY_BRIDGE_PKG := ./cmd/legacy-bridge

ECU_HOST ?= root@10.25.1.33
ECU_DIR ?= /home/applications/inv-driver
LEGACY_BRIDGE_ECU_DIR ?= /home/applications/legacy-bridge

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

PROTOC ?= protoc

# proto target is a manual prerequisite. It is idempotent (overwrites
# the same wire/busmgr.pb.go) but we deliberately don't chain it into
# build/build-arm so cross-platform builds don't require protoc on
# every host. Run `make proto` after editing proto/busmgr.proto.
.PHONY: all build build-arm clean fmt vet test deploy proto \
        build-legacy-bridge build-legacy-bridge-arm deploy-legacy-bridge

all: build-arm build-legacy-bridge-arm

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

clean:
	rm -rf $(BUILD_DIR)
