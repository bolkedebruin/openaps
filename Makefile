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
RECOVERYD_PKG        := ./cmd/recoveryd
TLS_PROXY_PKG        := ./cmd/openaps-tls-proxy
MKIPK_PKG            := ./cmd/mkipk

INV_DRIVER_BIN       := $(BUILD_DIR)/inv-driver
INV_DRIVER_ARMV7     := $(BUILD_DIR)/inv-driver-armv7
ECU_WEB_BIN          := $(BUILD_DIR)/ecu-web
ECU_WEB_ARMV7        := $(BUILD_DIR)/ecu-web-armv7
ECU_ZB_BIN           := $(BUILD_DIR)/ecu-zb
ECU_ZB_ARMV7         := $(BUILD_DIR)/ecu-zb-armv7
ECU_SUNSPEC_BIN      := $(BUILD_DIR)/ecu-sunspec
ECU_SUNSPEC_ARMV7    := $(BUILD_DIR)/ecu-sunspec-armv7
RECOVERYD_BIN        := $(BUILD_DIR)/recoveryd
RECOVERYD_ARMV7      := $(BUILD_DIR)/recoveryd-armv7
TLS_PROXY_BIN        := $(BUILD_DIR)/openaps-tls-proxy
TLS_PROXY_ARMV7      := $(BUILD_DIR)/openaps-tls-proxy-armv7
MKIPK_BIN            := $(BUILD_DIR)/mkipk

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

# DROPBEAR_DIR holds extracted dropbear ARMv7 binaries.
# - package-sunspec-with-dropbear requires it as an explicit operator arg.
# - package-openaps auto-fetches into $(BUILD_DIR)/dropbear-armv7 if unset.
DROPBEAR_DIR ?= $(BUILD_DIR)/dropbear-armv7

.PHONY: all build-all build-all-arm \
        build-inv-driver build-inv-driver-arm \
        build-ecu-web build-ecu-web-arm \
        build-ecu-zb build-ecu-zb-arm \
        build-ecu-sunspec build-ecu-sunspec-arm \
        build-recoveryd build-recoveryd-arm \
        build-openaps-tls-proxy build-openaps-tls-proxy-arm \
        build-mkipk \
        deploy-inv-driver deploy-ecu-web deploy-ecu-zb deploy-ecu-sunspec \
        install-init-zb uninstall-init-zb \
        package-zb package-sunspec package-sunspec-with-dropbear \
        package-openaps package-all fetch-dropbear \
        ipk-all ipk-base ipk-inv-driver ipk-ecu-zb ipk-ecu-web ipk-ecu-sunspec \
        ipk-tls-proxy ipk-dropbear package-ipks package-bootstrap \
        web web-test proto \
        test vet fmt clean

# Default — build every ARMv7 binary.
all: build-all-arm

# ---------------- host builds ----------------

build-all: build-inv-driver build-ecu-web build-ecu-zb build-ecu-sunspec build-recoveryd build-openaps-tls-proxy

build-inv-driver:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(INV_DRIVER_BIN) $(INV_DRIVER_PKG)

build-recoveryd:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(RECOVERYD_BIN) $(RECOVERYD_PKG)

build-openaps-tls-proxy:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(TLS_PROXY_BIN) $(TLS_PROXY_PKG)

build-ecu-web:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(ECU_WEB_BIN) $(ECU_WEB_PKG)

build-ecu-zb:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(ECU_ZB_BIN) $(ECU_ZB_PKG)

build-ecu-sunspec:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(ECU_SUNSPEC_BIN) $(ECU_SUNSPEC_PKG)

# mkipk is host-only build tooling (assembles .ipk packages); never cross-built.
build-mkipk:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(MKIPK_BIN) $(MKIPK_PKG)

# ---------------- ARMv7 builds ----------------

build-all-arm: build-inv-driver-arm build-ecu-web-arm build-ecu-zb-arm build-ecu-sunspec-arm build-recoveryd-arm build-openaps-tls-proxy-arm

build-inv-driver-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS_ARM) -ldflags '$(LDFLAGS_ARM)' -o $(INV_DRIVER_ARMV7) $(INV_DRIVER_PKG)
	@echo "built $(INV_DRIVER_ARMV7) ($$(wc -c <$(INV_DRIVER_ARMV7)) bytes)"

build-recoveryd-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS_ARM) -ldflags '$(LDFLAGS_ARM)' -o $(RECOVERYD_ARMV7) $(RECOVERYD_PKG)
	@echo "built $(RECOVERYD_ARMV7) ($$(wc -c <$(RECOVERYD_ARMV7)) bytes)"

build-openaps-tls-proxy-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS_ARM) -ldflags '$(LDFLAGS_ARM)' -o $(TLS_PROXY_ARMV7) $(TLS_PROXY_PKG)
	@echo "built $(TLS_PROXY_ARMV7) ($$(wc -c <$(TLS_PROXY_ARMV7)) bytes)"

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

# proto is a manual prerequisite. It overwrites wire/busmgr.pb.go and
# wire/recoveryd.pb.go. Run `make proto` after editing a proto/*.proto.
proto:
	$(PROTOC) -I proto --go_out=wire --go_opt=paths=source_relative proto/busmgr.proto
	$(PROTOC) -I proto --go_out=wire --go_opt=paths=source_relative proto/recoveryd.proto

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

fetch-dropbear: $(DROPBEAR_DIR)/dropbear

$(DROPBEAR_DIR)/dropbear:
	@./packaging/fetch-dropbear.sh $(DROPBEAR_DIR)

package-all: package-zb package-sunspec

# package-openaps — single master installer tarball for brownfield ECU
# install via stock POST /index.php/management/exec_upgrade_ecu_app.
#
# Layout:
#   openaps-$(VERSION)-ecu.tar.bz2
#   ├── update_localweb/
#   │   └── assist                       (master orchestrator)
#   └── update/
#       ├── applications/{inv-driver,ecu-zb,ecu-web,ecu-sunspec}
#       ├── rcS.d/{S48,S53,S54,S98,S99}-*
#       ├── dropbear/                    (optional, DROPBEAR_DIR=)
#       ├── etc-openaps/{release.pub,release.pub.README,git-sha}
#       ├── etc-inv-driver/settings.json.sample
#       └── openaps-rollback             (rollback CLI)
#
# Deterministic: --sort by file name, fixed mtime (git commit ts by default).
SOURCE_DATE_EPOCH ?= $(shell git log -1 --format=%ct 2>/dev/null || echo 1700000000)
GIT_SHA           ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
OPENAPS_PKG_NAME  := openaps-$(VERSION)-ecu.tar.bz2

package-openaps: build-all-arm $(DROPBEAR_DIR)/dropbear
	@echo "+ packaging openaps $(VERSION) (sha=$(GIT_SHA))"
	@rm -rf $(BUILD_DIR)/pkgroot-openaps
	@mkdir -p $(BUILD_DIR)/pkgroot-openaps/update_localweb
	@mkdir -p $(BUILD_DIR)/pkgroot-openaps/update/applications
	@mkdir -p $(BUILD_DIR)/pkgroot-openaps/update/rcS.d
	@mkdir -p $(BUILD_DIR)/pkgroot-openaps/update/etc-openaps
	@mkdir -p $(BUILD_DIR)/pkgroot-openaps/update/etc-inv-driver
	@sed 's|^VERSION="__OPENAPS_VERSION__"|VERSION="$(VERSION)"|' packaging/openaps-install > $(BUILD_DIR)/pkgroot-openaps/update_localweb/assist
	@chmod 0755 $(BUILD_DIR)/pkgroot-openaps/update_localweb/assist
	@cp $(INV_DRIVER_ARMV7)  $(BUILD_DIR)/pkgroot-openaps/update/applications/inv-driver
	@cp $(ECU_ZB_ARMV7)      $(BUILD_DIR)/pkgroot-openaps/update/applications/ecu-zb
	@cp $(ECU_WEB_ARMV7)     $(BUILD_DIR)/pkgroot-openaps/update/applications/ecu-web
	@cp $(ECU_SUNSPEC_ARMV7) $(BUILD_DIR)/pkgroot-openaps/update/applications/ecu-sunspec
	@cp $(RECOVERYD_ARMV7)   $(BUILD_DIR)/pkgroot-openaps/update/applications/recoveryd
	@chmod 0755 $(BUILD_DIR)/pkgroot-openaps/update/applications/*
	@cp packaging/S48-inv-driver $(BUILD_DIR)/pkgroot-openaps/update/rcS.d/S48-inv-driver
	@cp packaging/S53-ecu-zb     $(BUILD_DIR)/pkgroot-openaps/update/rcS.d/S53-ecu-zb
	@cp packaging/S54-ecu-web    $(BUILD_DIR)/pkgroot-openaps/update/rcS.d/S54-ecu-web
	@cp packaging/S97-recoveryd  $(BUILD_DIR)/pkgroot-openaps/update/rcS.d/S97-recoveryd
	@cp packaging/S98-dropbear   $(BUILD_DIR)/pkgroot-openaps/update/rcS.d/S98-dropbear
	@cp packaging/S99-sunspec    $(BUILD_DIR)/pkgroot-openaps/update/rcS.d/S99-sunspec
	@chmod 0755 $(BUILD_DIR)/pkgroot-openaps/update/rcS.d/*
	@if [ -n "$(DROPBEAR_DIR)" ] && [ -f "$(DROPBEAR_DIR)/dropbear" ]; then \
		mkdir -p $(BUILD_DIR)/pkgroot-openaps/update/dropbear; \
		for f in dropbear dropbearkey dropbearconvert dbclient authorized_keys; do \
			if [ -f "$(DROPBEAR_DIR)/$$f" ]; then \
				cp "$(DROPBEAR_DIR)/$$f" $(BUILD_DIR)/pkgroot-openaps/update/dropbear/; \
				echo "  +dropbear/$$f"; \
			fi; \
		done; \
		chmod 0755 $(BUILD_DIR)/pkgroot-openaps/update/dropbear/dropbear* 2>/dev/null || true; \
	else \
		echo "  (no DROPBEAR_DIR — installer will require pre-existing dropbear on :22)"; \
	fi
	@cp packaging/release.pub        $(BUILD_DIR)/pkgroot-openaps/update/etc-openaps/release.pub
	@cp packaging/release.pub.README $(BUILD_DIR)/pkgroot-openaps/update/etc-openaps/release.pub.README
	@echo "$(GIT_SHA)" > $(BUILD_DIR)/pkgroot-openaps/update/etc-openaps/git-sha
	@chmod 0644 $(BUILD_DIR)/pkgroot-openaps/update/etc-openaps/*
	@cp packaging/settings.json.sample $(BUILD_DIR)/pkgroot-openaps/update/etc-inv-driver/settings.json.sample
	@chmod 0644 $(BUILD_DIR)/pkgroot-openaps/update/etc-inv-driver/settings.json.sample
	@mkdir -p $(BUILD_DIR)/pkgroot-openaps/update/gridprofiles/profiles
	@cp gridprofiles-seed/profiles/*.json $(BUILD_DIR)/pkgroot-openaps/update/gridprofiles/profiles/
	@chmod 0644 $(BUILD_DIR)/pkgroot-openaps/update/gridprofiles/profiles/*.json
	@echo "  +grid profiles: $$(ls gridprofiles-seed/profiles/*.json | wc -l | tr -d ' ')"
	@cp packaging/openaps-rollback $(BUILD_DIR)/pkgroot-openaps/update/openaps-rollback
	@chmod 0755 $(BUILD_DIR)/pkgroot-openaps/update/openaps-rollback
	@# Deterministic tarball: name-sorted file list, fixed mtime via
	@# touch -t is portable across GNU/BSD tar; uid/gid flag-syntax diverges
	@# between GNU (--owner=N --group=N) and BSD (--uid N --gid N), so we
	@# skip the per-host uid/gid normalisation — extraction runs as root on
	@# the ECU and overwrites ownership anyway.
	@TS=$$(python3 -c "import datetime; print(datetime.datetime.utcfromtimestamp($(SOURCE_DATE_EPOCH)).strftime('%Y%m%d%H%M.%S'))" 2>/dev/null || date -u -r $(SOURCE_DATE_EPOCH) +%Y%m%d%H%M.%S 2>/dev/null || echo 202311140000.00); \
	find $(BUILD_DIR)/pkgroot-openaps -exec touch -t $$TS {} +
	@(cd $(BUILD_DIR)/pkgroot-openaps && \
		find . ! -path . | LC_ALL=C sort > /tmp/.openaps-files.lst && \
		tar -cjf ../$(OPENAPS_PKG_NAME) -T /tmp/.openaps-files.lst -n)
	@rm -f /tmp/.openaps-files.lst
	@rm -rf $(BUILD_DIR)/pkgroot-openaps
	@echo
	@echo "=== built $(BUILD_DIR)/$(OPENAPS_PKG_NAME) ==="
	@ls -lh $(BUILD_DIR)/$(OPENAPS_PKG_NAME)
	@(cd $(BUILD_DIR) && (sha256sum $(OPENAPS_PKG_NAME) 2>/dev/null || shasum -a 256 $(OPENAPS_PKG_NAME)))

# ---------------- .ipk packaging (opkg) ----------------
#
# Real Debian-format .ipk packages installable by opkg on the ECU. Each .ipk
# is an `ar` archive of: debian-binary, control.tar.gz, data.tar.gz, assembled
# by the host-only cmd/mkipk tool.
#
# opkg on this device enforces ONLY MD5Sum (it ignores SHA256sum and cannot
# verify signatures); authenticity comes from openaps-tls-proxy verifying the
# release-key RSA signature on the feed index, NOT from the .ipk itself.
#
# Metadata (control + maintainer scripts) lives under packaging/ipk/<pkg>/.
# Payload is staged into build/ipkroot/<pkg> by each target, then packed.
#
# Architecture: binaries -> armv7ahf-vfp-neon ; config/script-only -> all.
# The .ipk Version field is stamped from $(VERSION) at build time.

IPK_DIR        := $(BUILD_DIR)/ipk
IPKROOT        := $(BUILD_DIR)/ipkroot
IPK_META       := packaging/ipk
IPK_ARCH       := armv7ahf-vfp-neon

# stage_service,<pkg>,<armv7-binary-path>,<init-basename> — lay out a service
# package's data tree (ARMv7 daemon under /home/applications + its rcS.d init),
# generate its postinst from _service-postinst.in with the init path stamped in,
# then pack it. The service dir name is the package minus its openaps- prefix.
define stage_service
	@rm -rf $(IPKROOT)/$(1)
	@mkdir -p $(IPKROOT)/$(1)/home/applications/$(patsubst openaps-%,%,$(1))
	@mkdir -p $(IPKROOT)/$(1)/etc/rcS.d
	@cp $(2) $(IPKROOT)/$(1)/home/applications/$(patsubst openaps-%,%,$(1))/$(patsubst openaps-%,%,$(1))
	@chmod 0755 $(IPKROOT)/$(1)/home/applications/$(patsubst openaps-%,%,$(1))/$(patsubst openaps-%,%,$(1))
	@cp packaging/$(3) $(IPKROOT)/$(1)/etc/rcS.d/$(3)
	@chmod 0755 $(IPKROOT)/$(1)/etc/rcS.d/$(3)
	@mkdir -p $(IPK_DIR)
	@sed 's|__INIT__|/etc/rcS.d/$(3)|' $(IPK_META)/_service-postinst.in > $(IPK_DIR)/.gen-$(1)-postinst
	$(call call_mkipk,$(1),$(IPK_ARCH))
	@rm -f $(IPK_DIR)/.gen-$(1)-postinst
endef

# call_mkipk,<pkg>,<arch> — stamp control Version/Architecture, gather the
# package's maintainer scripts into a scripts dir (kept SEPARATE from the
# control file so mkipk emits ./control exactly once), and run mkipk to pack
# build/ipkroot/<pkg> into $(IPK_DIR)/<pkg>_<version>_<arch>.ipk.
define call_mkipk
	@echo "+ ipk: $(1) ($(2)) $(VERSION)"
	@rm -rf $(IPK_DIR)/.ctl-$(1)
	@mkdir -p $(IPK_DIR)/.ctl-$(1)/scripts $(IPK_DIR) $(IPKROOT)/$(1)
	@sed 's|^Version: __OPENAPS_VERSION__|Version: $(VERSION)|; s|^Architecture: .*|Architecture: $(2)|' \
		$(IPK_META)/$(1)/control > $(IPK_DIR)/.ctl-$(1)/control
	@chmod 0644 $(IPK_DIR)/.ctl-$(1)/control
	@for s in postinst preinst prerm postrm conffiles; do \
		if [ -f $(IPK_META)/$(1)/$$s ]; then \
			cp $(IPK_META)/$(1)/$$s $(IPK_DIR)/.ctl-$(1)/scripts/$$s; \
			[ "$$s" = conffiles ] && chmod 0644 $(IPK_DIR)/.ctl-$(1)/scripts/$$s || chmod 0755 $(IPK_DIR)/.ctl-$(1)/scripts/$$s; \
		fi; \
	done
	@if [ -f $(IPK_DIR)/.gen-$(1)-postinst ]; then \
		cp $(IPK_DIR)/.gen-$(1)-postinst $(IPK_DIR)/.ctl-$(1)/scripts/postinst; \
		chmod 0755 $(IPK_DIR)/.ctl-$(1)/scripts/postinst; \
	fi
	@rm -f $(IPK_DIR)/$(1)_$(VERSION)_$(2).ipk
	@$(MKIPK_BIN) \
		-control $(IPK_DIR)/.ctl-$(1)/control \
		-data    $(IPKROOT)/$(1) \
		-scripts $(IPK_DIR)/.ctl-$(1)/scripts \
		-out     $(IPK_DIR)/$(1)_$(VERSION)_$(2).ipk
	@rm -rf $(IPK_DIR)/.ctl-$(1)
	@ls -lh $(IPK_DIR)/$(1)_$(VERSION)_$(2).ipk
endef

ipk-all: ipk-base ipk-inv-driver ipk-ecu-zb ipk-ecu-web ipk-ecu-sunspec ipk-tls-proxy ipk-dropbear ipk-apsystems-stock

# (a) openaps-base — Architecture: all, no Depends. Ships release.pub +
#     openaps-rollback; postinst provisions settings.json from /etc/yuneng.
ipk-base: build-mkipk
	@rm -rf $(IPKROOT)/openaps-base
	@mkdir -p $(IPKROOT)/openaps-base/etc/openaps
	@mkdir -p $(IPKROOT)/openaps-base/etc/inv-driver
	@mkdir -p $(IPKROOT)/openaps-base/etc/rcS.d
	@mkdir -p $(IPKROOT)/openaps-base/usr/bin
	@cp packaging/release.pub $(IPKROOT)/openaps-base/etc/openaps/release.pub
	@chmod 0644 $(IPKROOT)/openaps-base/etc/openaps/release.pub
	@cp packaging/openaps-rollback $(IPKROOT)/openaps-base/usr/bin/openaps-rollback
	@chmod 0755 $(IPKROOT)/openaps-base/usr/bin/openaps-rollback
	@# S39 sets eth0's provisioned MAC before networking, so the box keeps its
	@# DHCP IP when the stock manager (macapp) is disabled. Harmless when stock
	@# is active. Shipped in base so it is present before apsystems-stock disable.
	@cp packaging/S39-openaps-macaddr $(IPKROOT)/openaps-base/etc/rcS.d/S39-openaps-macaddr
	@chmod 0755 $(IPKROOT)/openaps-base/etc/rcS.d/S39-openaps-macaddr
	$(call call_mkipk,openaps-base,all)

# apsystems-stock — wraps the stock firmware: installed = active, removed =
# disabled (prerm comments the manager launch + stops it; postinst restores).
# All control logic is in the package's prerm/postinst; the data is just a doc.
ipk-apsystems-stock: build-mkipk
	@rm -rf $(IPKROOT)/apsystems-stock
	@mkdir -p $(IPKROOT)/apsystems-stock/usr/share/doc/apsystems-stock
	@printf 'apsystems-stock: installed=stock active; remove to disable the stock\nmanager (reversible). See the package Description.\n' \
		> $(IPKROOT)/apsystems-stock/usr/share/doc/apsystems-stock/README
	@chmod 0644 $(IPKROOT)/apsystems-stock/usr/share/doc/apsystems-stock/README
	$(call call_mkipk,apsystems-stock,all)

# (b) openaps-inv-driver — armv7ahf-vfp-neon, Depends: openaps-base.
ipk-inv-driver: build-inv-driver-arm build-mkipk
	$(call stage_service,openaps-inv-driver,$(INV_DRIVER_ARMV7),S48-inv-driver)

# (c) openaps-ecu-zb — armv7ahf-vfp-neon, Depends: openaps-base, openaps-inv-driver.
ipk-ecu-zb: build-ecu-zb-arm build-mkipk
	$(call stage_service,openaps-ecu-zb,$(ECU_ZB_ARMV7),S53-ecu-zb)

# (d) openaps-ecu-web — armv7ahf-vfp-neon, Depends: openaps-base, openaps-inv-driver.
ipk-ecu-web: build-ecu-web-arm build-mkipk
	$(call stage_service,openaps-ecu-web,$(ECU_WEB_ARMV7),S54-ecu-web)

# (e) openaps-ecu-sunspec — armv7ahf-vfp-neon, Depends: openaps-base, openaps-inv-driver.
ipk-ecu-sunspec: build-ecu-sunspec-arm build-mkipk
	$(call stage_service,openaps-ecu-sunspec,$(ECU_SUNSPEC_ARMV7),S99-sunspec)

# (f) openaps-tls-proxy — armv7ahf-vfp-neon, Depends: none. Ships the proxy
#     binary + opkg feed config + the upstream feed.conf sample + the S47
#     rcS.d init that runs the proxy persistently; postinst seeds
#     /etc/openaps/feed.conf, points opkg at the loopback feed, and starts it.
ipk-tls-proxy: build-openaps-tls-proxy-arm build-mkipk
	@rm -rf $(IPKROOT)/openaps-tls-proxy
	@mkdir -p $(IPKROOT)/openaps-tls-proxy/home/applications/openaps-tls-proxy
	@mkdir -p $(IPKROOT)/openaps-tls-proxy/etc/rcS.d
	@mkdir -p $(IPKROOT)/openaps-tls-proxy/etc/opkg
	@mkdir -p $(IPKROOT)/openaps-tls-proxy/etc/openaps
	@cp $(TLS_PROXY_ARMV7) $(IPKROOT)/openaps-tls-proxy/home/applications/openaps-tls-proxy/openaps-tls-proxy
	@chmod 0755 $(IPKROOT)/openaps-tls-proxy/home/applications/openaps-tls-proxy/openaps-tls-proxy
	@# Ship the opkg feed config + operator-editable upstream as their final-path
	@# data files so opkg tracks them as conffiles (preserved on upgrade); the
	@# conffiles manifest lists these exact paths.
	@cp packaging/opkg-openaps.conf $(IPKROOT)/openaps-tls-proxy/etc/opkg/openaps.conf
	@chmod 0644 $(IPKROOT)/openaps-tls-proxy/etc/opkg/openaps.conf
	@cp packaging/feed.conf.sample $(IPKROOT)/openaps-tls-proxy/etc/openaps/feed.conf
	@chmod 0644 $(IPKROOT)/openaps-tls-proxy/etc/openaps/feed.conf
	@cp packaging/S47-openaps-tls-proxy $(IPKROOT)/openaps-tls-proxy/etc/rcS.d/S47-openaps-tls-proxy
	@chmod 0755 $(IPKROOT)/openaps-tls-proxy/etc/rcS.d/S47-openaps-tls-proxy
	$(call call_mkipk,openaps-tls-proxy,$(IPK_ARCH))

# (g) openaps-dropbear — armv7ahf-vfp-neon, Depends: none. Bundles the
#     dropbear ARMv7 binaries (fetched into $(DROPBEAR_DIR)) + S98 init script.
ipk-dropbear: build-mkipk $(DROPBEAR_DIR)/dropbear
	@rm -rf $(IPKROOT)/openaps-dropbear
	@mkdir -p $(IPKROOT)/openaps-dropbear/usr/local/sbin
	@mkdir -p $(IPKROOT)/openaps-dropbear/etc/rcS.d
	@for f in dropbear dropbearkey dbclient dropbearconvert; do \
		if [ -f "$(DROPBEAR_DIR)/$$f" ]; then \
			cp "$(DROPBEAR_DIR)/$$f" $(IPKROOT)/openaps-dropbear/usr/local/sbin/$$f; \
			chmod 0755 $(IPKROOT)/openaps-dropbear/usr/local/sbin/$$f; \
		fi; \
	done
	@cp packaging/S98-dropbear $(IPKROOT)/openaps-dropbear/etc/rcS.d/S98-dropbear
	@chmod 0755 $(IPKROOT)/openaps-dropbear/etc/rcS.d/S98-dropbear
	$(call call_mkipk,openaps-dropbear,$(IPK_ARCH))

# package-ipks — build all 7 .ipks, then mirror them into build/ipks/ (the dir
# the bootstrap tarball and a published feed both consume).
package-ipks: ipk-all
	@rm -rf $(BUILD_DIR)/ipks
	@mkdir -p $(BUILD_DIR)/ipks
	@cp $(IPK_DIR)/*_$(VERSION)_*.ipk $(BUILD_DIR)/ipks/
	@echo "=== built .ipks ==="
	@ls -lh $(BUILD_DIR)/ipks/*.ipk

# ---------------- bootstrap tarball (stock exec_upgrade_ecu_app foothold) ----
#
# openaps-bootstrap-<ver>.tar.gz delivered via the stock hidden endpoint
# (exec_upgrade_ecu_app extracts the tarball and runs the root-level "assist").
#
# Layout (assist at the tarball ROOT, siblings beside it):
#   assist                                                  (orchestrator)
#   ipks/openaps-dropbear_<ver>_armv7ahf-vfp-neon.ipk       (carries the root password)
#   ipks/openaps-tls-proxy_<ver>_armv7ahf-vfp-neon.ipk
#   ipks/apsystems-stock_<ver>_all.ipk
#   release.pub          -> /etc/openaps/release.pub
#   authorized_keys      -> /home/root/.ssh/authorized_keys (optional)
#   opkg-openaps.conf    -> /etc/opkg/openaps.conf
#   root.shadow.hash     ($6$ SHA-512 crypt hash, baked from ROOT_PW)
#
# REQUIRED make vars:
#   ROOT_PW          — plaintext root password (default convention "openaps");
#                      hashed at build with `openssl passwd -6` into
#                      root.shadow.hash and applied once by the assist. FAILS if
#                      unset — a box must get a KNOWN password, or disabling stock
#                      (which stops idwriter too) would lock it out.
# OPTIONAL make vars:
#   AUTHORIZED_KEYS  — path to an SSH public-key file to bundle. Omitted for a
#                      generally distributed bootstrap: first login is root + the
#                      baked password, after which the operator adds their key.
#
# ROOT_PW reaches the recipe via the environment so it is hashed through
# `openssl passwd -6 -stdin` (never on argv / in the process list).
# BusyBox-compatible tar: gzip, no -P, assist at the root.
BOOTSTRAP_PKG_NAME := openaps-bootstrap-$(VERSION).tar.gz
AUTHORIZED_KEYS    ?=
ROOT_PW            ?=

package-bootstrap: export ROOT_PW := $(ROOT_PW)
package-bootstrap: ipk-dropbear ipk-tls-proxy ipk-apsystems-stock
	@[ -n "$$ROOT_PW" ] || { echo "ERROR: ROOT_PW is required (e.g. ROOT_PW=openaps) — a bootstrap with no known root password could brick the box when stock is disabled"; exit 1; }
	@echo "+ packaging openaps-bootstrap $(VERSION)"
	@rm -rf $(BUILD_DIR)/pkgroot-bootstrap
	@mkdir -p $(BUILD_DIR)/pkgroot-bootstrap/ipks
	@sed 's|^VERSION="__OPENAPS_VERSION__"|VERSION="$(VERSION)"|' packaging/openaps-bootstrap/assist > $(BUILD_DIR)/pkgroot-bootstrap/assist
	@chmod 0755 $(BUILD_DIR)/pkgroot-bootstrap/assist
	@cp $(IPK_DIR)/openaps-dropbear_$(VERSION)_$(IPK_ARCH).ipk   $(BUILD_DIR)/pkgroot-bootstrap/ipks/
	@cp $(IPK_DIR)/openaps-tls-proxy_$(VERSION)_$(IPK_ARCH).ipk  $(BUILD_DIR)/pkgroot-bootstrap/ipks/
	@cp $(IPK_DIR)/apsystems-stock_$(VERSION)_all.ipk           $(BUILD_DIR)/pkgroot-bootstrap/ipks/
	@cp packaging/release.pub       $(BUILD_DIR)/pkgroot-bootstrap/release.pub
	@cp packaging/opkg-openaps.conf $(BUILD_DIR)/pkgroot-bootstrap/opkg-openaps.conf
	@chmod 0644 $(BUILD_DIR)/pkgroot-bootstrap/release.pub $(BUILD_DIR)/pkgroot-bootstrap/opkg-openaps.conf
	@# Optional bundled key. Omitted -> first login is root + the baked password.
	@if [ -n "$(AUTHORIZED_KEYS)" ]; then \
		[ -f "$(AUTHORIZED_KEYS)" ] || { echo "ERROR: AUTHORIZED_KEYS file not found: $(AUTHORIZED_KEYS)"; exit 1; }; \
		cp "$(AUTHORIZED_KEYS)" $(BUILD_DIR)/pkgroot-bootstrap/authorized_keys; \
		chmod 0644 $(BUILD_DIR)/pkgroot-bootstrap/authorized_keys; \
		echo "  + authorized_keys (bundled operator key)"; \
	else \
		echo "  (no AUTHORIZED_KEYS bundled — first login is root + the baked password)"; \
	fi
	@printf '%s' "$$ROOT_PW" | openssl passwd -6 -stdin > $(BUILD_DIR)/pkgroot-bootstrap/root.shadow.hash; \
		case "$$(cat $(BUILD_DIR)/pkgroot-bootstrap/root.shadow.hash)" in \
			\$$6\$$*) ;; \
			*) echo "ERROR: root.shadow.hash is not a \$$6\$$ SHA-512 crypt hash"; exit 1 ;; \
		esac
	@chmod 0600 $(BUILD_DIR)/pkgroot-bootstrap/root.shadow.hash
	@(cd $(BUILD_DIR)/pkgroot-bootstrap && tar -czf ../$(BOOTSTRAP_PKG_NAME) .)
	@rm -rf $(BUILD_DIR)/pkgroot-bootstrap
	@echo "=== built $(BUILD_DIR)/$(BOOTSTRAP_PKG_NAME) ==="
	@ls -lh $(BUILD_DIR)/$(BOOTSTRAP_PKG_NAME)

# ---------------- common ----------------

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
