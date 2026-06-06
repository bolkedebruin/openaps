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
        deploy-inv-driver deploy-ecu-web deploy-ecu-zb deploy-ecu-sunspec \
        install-init-zb uninstall-init-zb \
        package-zb package-sunspec package-sunspec-with-dropbear \
        package-openaps package-all fetch-dropbear \
        web web-test proto \
        test vet fmt clean

# Default — build every ARMv7 binary.
all: build-all-arm

# ---------------- host builds ----------------

build-all: build-inv-driver build-ecu-web build-ecu-zb build-ecu-sunspec build-recoveryd

build-inv-driver:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(INV_DRIVER_BIN) $(INV_DRIVER_PKG)

build-recoveryd:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_HOST)' -o $(RECOVERYD_BIN) $(RECOVERYD_PKG)

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

build-all-arm: build-inv-driver-arm build-ecu-web-arm build-ecu-zb-arm build-ecu-sunspec-arm build-recoveryd-arm

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

# ---------------- common ----------------

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
