# OpenAPS monorepo Makefile.
#
# Produces four ARMv7 binaries (TI AM335x, glibc 2.15, CGO disabled):
#   - inv-driver       — bus owner, codec, ingest, gridprofile, IPC server
#   - ecu-web          — VRM-style console (HTTP2/SSE), embedded SPA
#   - ecu-zb           — ZigBee modem proxy + bus manager
#   - ecu-sunspec      — Modbus/TCP + RTU SunSpec frontend (subscriber path)

BUILD_DIR := build

INV_DRIVER_PKG       := ./cmd/inv-driver
ECU_WEB_PKG          := ./cmd/ecu-web
ECU_ZB_PKG           := ./cmd/ecu-zb
ECU_SUNSPEC_PKG      := ./cmd/ecu-sunspec

INV_DRIVER_BIN       := $(BUILD_DIR)/inv-driver
INV_DRIVER_ARMV7     := $(BUILD_DIR)/inv-driver-armv7
ECU_WEB_BIN          := $(BUILD_DIR)/ecu-web
ECU_WEB_ARMV7        := $(BUILD_DIR)/ecu-web-armv7
ECU_ZB_BIN           := $(BUILD_DIR)/ecu-zb
ECU_ZB_ARMV7         := $(BUILD_DIR)/ecu-zb-armv7
ECU_SUNSPEC_BIN      := $(BUILD_DIR)/ecu-sunspec
ECU_SUNSPEC_ARMV7    := $(BUILD_DIR)/ecu-sunspec-armv7

ECU_WEB_DIR_SRC      := cmd/ecu-web/web

# ECU_HOST is REQUIRED for any deploy/install target. There is no default.
ECU_HOST ?=
INV_DRIVER_ECU_DIR   ?= /home/applications/inv-driver
ECU_WEB_ECU_DIR      ?= /home/applications/ecu-web
ECU_ZB_ECU_DIR       ?= /home/applications/ecu-zb
ECU_SUNSPEC_ECU_DIR  ?= /home/applications/ecu-sunspec

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

LDFLAGS_HOST := -X main.version=$(VERSION)
LDFLAGS_ARM  := -s -w -X main.version=$(VERSION)
GOFLAGS_ARM  := -trimpath

PROTOC ?= protoc

# DROPBEAR_DIR is optional for package-sunspec-with-dropbear (preserves
# the ecu-sunspec optional dropbear bundle path).
DROPBEAR_DIR ?=

.PHONY: all build-all build-all-arm \
        build-inv-driver build-inv-driver-arm \
        build-ecu-web build-ecu-web-arm \
        build-ecu-zb build-ecu-zb-arm \
        build-ecu-sunspec build-ecu-sunspec-arm \
        deploy-inv-driver deploy-ecu-web deploy-ecu-zb deploy-ecu-sunspec \
        install-init-zb uninstall-init-zb \
        package-zb package-sunspec package-sunspec-with-dropbear \
        package-all fetch-dropbear \
        web web-test proto \
        test vet fmt clean

# Default — build every ARMv7 binary.
all: build-all-arm

# ---------------- host builds ----------------

build-all: build-inv-driver build-ecu-web build-ecu-zb build-ecu-sunspec

build-inv-driver:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(INV_DRIVER_BIN) $(INV_DRIVER_PKG)

build-ecu-web:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(ECU_WEB_BIN) $(ECU_WEB_PKG)

build-ecu-zb:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(ECU_ZB_BIN) $(ECU_ZB_PKG)

build-ecu-sunspec:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(ECU_SUNSPEC_BIN) $(ECU_SUNSPEC_PKG)

# ---------------- ARMv7 builds ----------------

build-all-arm: build-inv-driver-arm build-ecu-web-arm build-ecu-zb-arm build-ecu-sunspec-arm

build-inv-driver-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS_ARM) -ldflags '$(LDFLAGS_ARM)' -o $(INV_DRIVER_ARMV7) $(INV_DRIVER_PKG)
	@echo "built $(INV_DRIVER_ARMV7) ($$(wc -c <$(INV_DRIVER_ARMV7)) bytes)"

build-ecu-web-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS_ARM) -ldflags '$(LDFLAGS_ARM)' -o $(ECU_WEB_ARMV7) $(ECU_WEB_PKG)
	@echo "built $(ECU_WEB_ARMV7) ($$(wc -c <$(ECU_WEB_ARMV7)) bytes)"

build-ecu-zb-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS_ARM) -ldflags '$(LDFLAGS_ARM)' -o $(ECU_ZB_ARMV7) $(ECU_ZB_PKG)
	@echo "built $(ECU_ZB_ARMV7) ($$(wc -c <$(ECU_ZB_ARMV7)) bytes)"

build-ecu-sunspec-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS_ARM) -ldflags '$(LDFLAGS_ARM)' -o $(ECU_SUNSPEC_ARMV7) $(ECU_SUNSPEC_PKG)
	@echo "built $(ECU_SUNSPEC_ARMV7) ($$(wc -c <$(ECU_SUNSPEC_ARMV7)) bytes)"

# ---------------- proto + web ----------------

# proto is a manual prerequisite. It overwrites wire/busmgr.pb.go.
# Run `make proto` after editing proto/busmgr.proto.
proto:
	$(PROTOC) -I proto --go_out=wire --go_opt=paths=source_relative proto/busmgr.proto

# web rebuilds the embedded SPA bundle. dist/ is committed so cross builds
# don't require Bun. Run `make web` after editing the frontend.
web:
	cd $(ECU_WEB_DIR_SRC) && bun install && bun run build

web-test:
	cd $(ECU_WEB_DIR_SRC) && bun run cooldown && bun test

# ---------------- deploys (require ECU_HOST) ----------------

define require_ecu_host
	@[ -n "$(ECU_HOST)" ] || { echo "ECU_HOST is required (e.g. make $@ ECU_HOST=root@<ip>)"; exit 1; }
endef

deploy-inv-driver: build-inv-driver-arm
	$(call require_ecu_host)
	ssh $(ECU_HOST) 'mkdir -p $(INV_DRIVER_ECU_DIR)'
	ssh $(ECU_HOST) 'cat > $(INV_DRIVER_ECU_DIR)/inv-driver.new' < $(INV_DRIVER_ARMV7)
	ssh $(ECU_HOST) 'chmod +x $(INV_DRIVER_ECU_DIR)/inv-driver.new && mv $(INV_DRIVER_ECU_DIR)/inv-driver.new $(INV_DRIVER_ECU_DIR)/inv-driver'
	@echo "deployed to $(ECU_HOST):$(INV_DRIVER_ECU_DIR)/inv-driver"

deploy-ecu-web: build-ecu-web-arm
	$(call require_ecu_host)
	ssh $(ECU_HOST) 'mkdir -p $(ECU_WEB_ECU_DIR)'
	ssh $(ECU_HOST) 'cat > $(ECU_WEB_ECU_DIR)/ecu-web.new' < $(ECU_WEB_ARMV7)
	ssh $(ECU_HOST) 'chmod +x $(ECU_WEB_ECU_DIR)/ecu-web.new && mv $(ECU_WEB_ECU_DIR)/ecu-web.new $(ECU_WEB_ECU_DIR)/ecu-web'
	@echo "deployed to $(ECU_HOST):$(ECU_WEB_ECU_DIR)/ecu-web"

deploy-ecu-zb: build-ecu-zb-arm
	$(call require_ecu_host)
	ssh $(ECU_HOST) 'mkdir -p $(ECU_ZB_ECU_DIR)/run $(ECU_ZB_ECU_DIR)/log'
	ssh $(ECU_HOST) 'cat > $(ECU_ZB_ECU_DIR)/ecu-zb.new' < $(ECU_ZB_ARMV7)
	ssh $(ECU_HOST) 'chmod +x $(ECU_ZB_ECU_DIR)/ecu-zb.new && mv $(ECU_ZB_ECU_DIR)/ecu-zb.new $(ECU_ZB_ECU_DIR)/ecu-zb'
	@echo "deployed to $(ECU_HOST):$(ECU_ZB_ECU_DIR)/ecu-zb"

deploy-ecu-sunspec: build-ecu-sunspec-arm
	$(call require_ecu_host)
	ssh $(ECU_HOST) 'mkdir -p $(ECU_SUNSPEC_ECU_DIR)'
	ssh $(ECU_HOST) 'cat > $(ECU_SUNSPEC_ECU_DIR)/ecu-sunspec.new' < $(ECU_SUNSPEC_ARMV7)
	ssh $(ECU_HOST) 'chmod +x $(ECU_SUNSPEC_ECU_DIR)/ecu-sunspec.new && mv $(ECU_SUNSPEC_ECU_DIR)/ecu-sunspec.new $(ECU_SUNSPEC_ECU_DIR)/ecu-sunspec'
	@echo "deployed to $(ECU_HOST):$(ECU_SUNSPEC_ECU_DIR)/ecu-sunspec"

# Install the BusyBox init script that starts ecu-zb at S53 in rcS.d.
install-init-zb:
	$(call require_ecu_host)
	ssh $(ECU_HOST) 'cat > /etc/rcS.d/S53-ecu-zb' < packaging/S53-ecu-zb
	ssh $(ECU_HOST) 'chmod +x /etc/rcS.d/S53-ecu-zb && ls -la /etc/rcS.d/S53-ecu-zb'
	@echo "init script installed. Reboot the ECU to start ecu-zb at boot."

uninstall-init-zb:
	$(call require_ecu_host)
	ssh $(ECU_HOST) 'rm -f /etc/rcS.d/S53-ecu-zb'
	@echo "init script removed"

# ---------------- packaging (apsystems-style upgrade tarballs) ----------------

# package-zb — apsystems-ecu-zb-<ver>.tar.bz2 for the local upgrade endpoint.
#
# Layout (as required by /index.php/management/exec_upgrade_ecu_app):
#
#   apsystems-ecu-zb-<ver>.tar.bz2
#   ├── update_localweb/
#   │   └── assist                   (shell installer)
#   └── update/
#       ├── applications/
#       │   └── ecu-zb               (ARMv7 binary)
#       └── etc-rcS-d/
#           └── S53-ecu-zb           (BusyBox rcS.d init script)
package-zb: build-ecu-zb-arm
	@echo "+ packaging ecu-zb $(VERSION)"
	@rm -rf $(BUILD_DIR)/pkgroot-zb
	@mkdir -p $(BUILD_DIR)/pkgroot-zb/update_localweb
	@mkdir -p $(BUILD_DIR)/pkgroot-zb/update/applications
	@mkdir -p $(BUILD_DIR)/pkgroot-zb/update/etc-rcS-d
	@cp $(ECU_ZB_ARMV7) $(BUILD_DIR)/pkgroot-zb/update/applications/ecu-zb
	@chmod 0755 $(BUILD_DIR)/pkgroot-zb/update/applications/ecu-zb
	@cp packaging/assist-zb $(BUILD_DIR)/pkgroot-zb/update_localweb/assist
	@chmod 0755 $(BUILD_DIR)/pkgroot-zb/update_localweb/assist
	@cp packaging/S53-ecu-zb $(BUILD_DIR)/pkgroot-zb/update/etc-rcS-d/S53-ecu-zb
	@chmod 0755 $(BUILD_DIR)/pkgroot-zb/update/etc-rcS-d/S53-ecu-zb
	@(cd $(BUILD_DIR)/pkgroot-zb && tar -cjf ../apsystems-ecu-zb-$(VERSION).tar.bz2 .)
	@rm -rf $(BUILD_DIR)/pkgroot-zb
	@ls -lh $(BUILD_DIR)/apsystems-ecu-zb-$(VERSION).tar.bz2

# package-sunspec — apsystems-sunspec-<ver>.tar.bz2.
#
#   apsystems-sunspec-<ver>.tar.bz2
#   ├── update_localweb/
#   │   └── assist                       (shell installer)
#   └── update/
#       ├── applications/
#       │   └── ecu-sunspec               (ARMv7 binary)
#       ├── etc-init-d/
#       │   └── S99-sunspec               (BusyBox init script)
#       └── etc-sunspec/
#           ├── sunspec.json
#           └── sunspec-nameplate.json
package-sunspec: build-ecu-sunspec-arm
	@echo "+ packaging ecu-sunspec $(VERSION)"
	@rm -rf $(BUILD_DIR)/pkgroot-sunspec
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update_localweb
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update/applications
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update/etc-init-d
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec
	@cp $(ECU_SUNSPEC_ARMV7) $(BUILD_DIR)/pkgroot-sunspec/update/applications/ecu-sunspec
	@chmod 0755 $(BUILD_DIR)/pkgroot-sunspec/update/applications/ecu-sunspec
	@cp packaging/assist-sunspec $(BUILD_DIR)/pkgroot-sunspec/update_localweb/assist
	@chmod 0755 $(BUILD_DIR)/pkgroot-sunspec/update_localweb/assist
	@cp packaging/S99-sunspec $(BUILD_DIR)/pkgroot-sunspec/update/etc-init-d/S99-sunspec
	@chmod 0755 $(BUILD_DIR)/pkgroot-sunspec/update/etc-init-d/S99-sunspec
	@cp packaging/sunspec.json $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec.json
	@chmod 0644 $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec.json
	@cp packaging/nameplate.json $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec-nameplate.json
	@chmod 0644 $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec-nameplate.json
	@(cd $(BUILD_DIR)/pkgroot-sunspec && tar -cjf ../apsystems-sunspec-$(VERSION).tar.bz2 .)
	@rm -rf $(BUILD_DIR)/pkgroot-sunspec
	@ls -lh $(BUILD_DIR)/apsystems-sunspec-$(VERSION).tar.bz2

# package-sunspec-with-dropbear — package-sunspec + dropbear binaries
# staged from $(DROPBEAR_DIR).
package-sunspec-with-dropbear: build-ecu-sunspec-arm
	@if [ -z "$(DROPBEAR_DIR)" ]; then \
		echo "ERROR: set DROPBEAR_DIR=/path/to/dir-containing-dropbear-binaries"; \
		exit 1; \
	fi
	@if [ ! -f "$(DROPBEAR_DIR)/dropbear" ]; then \
		echo "ERROR: $(DROPBEAR_DIR)/dropbear not found"; exit 1; \
	fi
	@echo "+ packaging ecu-sunspec $(VERSION) with dropbear from $(DROPBEAR_DIR)"
	@rm -rf $(BUILD_DIR)/pkgroot-sunspec
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update_localweb
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update/applications
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update/etc-init-d
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec
	@mkdir -p $(BUILD_DIR)/pkgroot-sunspec/update/dropbear
	@cp $(ECU_SUNSPEC_ARMV7) $(BUILD_DIR)/pkgroot-sunspec/update/applications/ecu-sunspec
	@chmod 0755 $(BUILD_DIR)/pkgroot-sunspec/update/applications/ecu-sunspec
	@cp packaging/assist-sunspec $(BUILD_DIR)/pkgroot-sunspec/update_localweb/assist
	@chmod 0755 $(BUILD_DIR)/pkgroot-sunspec/update_localweb/assist
	@cp packaging/S99-sunspec $(BUILD_DIR)/pkgroot-sunspec/update/etc-init-d/S99-sunspec
	@cp packaging/S98-dropbear $(BUILD_DIR)/pkgroot-sunspec/update/etc-init-d/S98-dropbear
	@chmod 0755 $(BUILD_DIR)/pkgroot-sunspec/update/etc-init-d/*
	@cp packaging/sunspec.json $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec.json
	@chmod 0644 $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec.json
	@cp packaging/nameplate.json $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec-nameplate.json
	@chmod 0644 $(BUILD_DIR)/pkgroot-sunspec/update/etc-sunspec/sunspec-nameplate.json
	@for f in dropbear dropbearkey dropbearconvert dbclient scp authorized_keys; do \
		[ -f "$(DROPBEAR_DIR)/$$f" ] && cp "$(DROPBEAR_DIR)/$$f" $(BUILD_DIR)/pkgroot-sunspec/update/dropbear/ && echo "  +dropbear/$$f" || true; \
	done
	@chmod 0755 $(BUILD_DIR)/pkgroot-sunspec/update/dropbear/dropbear* 2>/dev/null || true
	@(cd $(BUILD_DIR)/pkgroot-sunspec && tar -cjf ../apsystems-sunspec-$(VERSION)-dropbear.tar.bz2 .)
	@rm -rf $(BUILD_DIR)/pkgroot-sunspec
	@ls -lh $(BUILD_DIR)/apsystems-sunspec-$(VERSION)-dropbear.tar.bz2

fetch-dropbear:
	@./packaging/fetch-dropbear.sh $(BUILD_DIR)/dropbear-armv7

package-all: package-zb package-sunspec

# ---------------- common ----------------

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
