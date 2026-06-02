BIN     := ecu-sunspec
PKG     := ./cmd/ecu-sunspec
DIST    := dist
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Build flags: pure Go, stripped, optimized.
LDFLAGS := -s -w
GOFLAGS := -trimpath -ldflags '$(LDFLAGS)'

# Optional dropbear bundle. Override on the command line:
#   make package-with-dropbear DROPBEAR_DIR=/path/to/dropbear
# The directory must contain at least:
#   dropbear, dropbearkey   (ARM 32-bit ELFs that work on the ECU's glibc 2.15)
# Optionally:
#   dropbearconvert, dbclient
#   authorized_keys         (one or more SSH public keys to install for root)
#
# Dropbear binaries are NOT shipped in this repo. Build or extract them
# separately for armv7 + glibc 2.15 (Debian wheezy 2012.55 binaries work).
DROPBEAR_DIR ?=

.PHONY: all test ecu sidecar mac clean package package-with-dropbear fetch-dropbear

all: ecu sidecar

# ECU: AM335x, ARMv7, glibc 2.15. CGO disabled — modernc.org/sqlite is pure Go.
ecu: $(DIST)/$(BIN)-linux-armv7

$(DIST)/$(BIN)-linux-armv7:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build $(GOFLAGS) -o $@ $(PKG)
	@ls -lh $@

# sidecar: x86_64 sidecar host (Synology, generic Linux server, etc.).
sidecar: $(DIST)/$(BIN)-linux-amd64

$(DIST)/$(BIN)-linux-amd64:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build $(GOFLAGS) -o $@ $(PKG)
	@ls -lh $@

# Local development build.
mac: $(DIST)/$(BIN)-darwin
$(DIST)/$(BIN)-darwin:
	@mkdir -p $(DIST)
	go build $(GOFLAGS) -o $@ $(PKG)

test:
	go test ./...

# package — build the deployable tarball that the ECU's local upgrade
# endpoint expects.
#
# Layout produced (as required by /index.php/management/exec_upgrade_ecu_app):
#
#   apsystems-sunspec-<ver>.tar.bz2
#   ├── update_localweb/
#   │   └── assist                       (shell installer; PHP runs this)
#   └── update/
#       ├── applications/
#       │   └── ecu-sunspec               (ARMv7 binary)
#       └── etc-init-d/
#           └── S99-sunspec               (BusyBox init script)
#
# Deploy:
#   curl -X POST -F file=@dist/apsystems-sunspec-<ver>.tar.bz2 \
#        http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
package: ecu
	@echo "+ packaging $(BIN) $(VERSION)"
	@rm -rf $(DIST)/pkgroot
	@mkdir -p $(DIST)/pkgroot/update_localweb
	@mkdir -p $(DIST)/pkgroot/update/applications
	@mkdir -p $(DIST)/pkgroot/update/etc-init-d
	@mkdir -p $(DIST)/pkgroot/update/etc-sunspec
	@cp $(DIST)/$(BIN)-linux-armv7 $(DIST)/pkgroot/update/applications/ecu-sunspec
	@chmod 0755 $(DIST)/pkgroot/update/applications/ecu-sunspec
	@cp packaging/assist $(DIST)/pkgroot/update_localweb/assist
	@chmod 0755 $(DIST)/pkgroot/update_localweb/assist
	@cp packaging/S99-sunspec $(DIST)/pkgroot/update/etc-init-d/S99-sunspec
	@chmod 0755 $(DIST)/pkgroot/update/etc-init-d/S99-sunspec
	@cp packaging/sunspec.json $(DIST)/pkgroot/update/etc-sunspec/sunspec.json
	@chmod 0644 $(DIST)/pkgroot/update/etc-sunspec/sunspec.json
	@cp packaging/nameplate.json $(DIST)/pkgroot/update/etc-sunspec/sunspec-nameplate.json
	@chmod 0644 $(DIST)/pkgroot/update/etc-sunspec/sunspec-nameplate.json
	@(cd $(DIST)/pkgroot && tar -cjf ../apsystems-sunspec-$(VERSION).tar.bz2 .)
	@rm -rf $(DIST)/pkgroot
	@ls -lh $(DIST)/apsystems-sunspec-$(VERSION).tar.bz2

# package-with-dropbear — same as `package`, plus dropbear binaries staged
# from $(DROPBEAR_DIR). Installs SSH server on the ECU as part of the deploy.
#
#   make package-with-dropbear DROPBEAR_DIR=/path/to/dropbear-armv7
#
# Resulting tarball is named apsystems-sunspec-<ver>-dropbear.tar.bz2.
package-with-dropbear: ecu
	@if [ -z "$(DROPBEAR_DIR)" ]; then \
		echo "ERROR: set DROPBEAR_DIR=/path/to/dir-containing-dropbear-binaries"; \
		echo "       e.g. make package-with-dropbear DROPBEAR_DIR=~/dropbear-armv7"; \
		exit 1; \
	fi
	@if [ ! -f "$(DROPBEAR_DIR)/dropbear" ]; then \
		echo "ERROR: $(DROPBEAR_DIR)/dropbear not found"; exit 1; \
	fi
	@echo "+ packaging $(BIN) $(VERSION) with dropbear from $(DROPBEAR_DIR)"
	@rm -rf $(DIST)/pkgroot
	@mkdir -p $(DIST)/pkgroot/update_localweb
	@mkdir -p $(DIST)/pkgroot/update/applications
	@mkdir -p $(DIST)/pkgroot/update/etc-init-d
	@mkdir -p $(DIST)/pkgroot/update/etc-sunspec
	@mkdir -p $(DIST)/pkgroot/update/dropbear
	@cp $(DIST)/$(BIN)-linux-armv7 $(DIST)/pkgroot/update/applications/ecu-sunspec
	@chmod 0755 $(DIST)/pkgroot/update/applications/ecu-sunspec
	@cp packaging/assist $(DIST)/pkgroot/update_localweb/assist
	@chmod 0755 $(DIST)/pkgroot/update_localweb/assist
	@cp packaging/S99-sunspec $(DIST)/pkgroot/update/etc-init-d/S99-sunspec
	@cp packaging/S98-dropbear $(DIST)/pkgroot/update/etc-init-d/S98-dropbear
	@chmod 0755 $(DIST)/pkgroot/update/etc-init-d/*
	@cp packaging/sunspec.json $(DIST)/pkgroot/update/etc-sunspec/sunspec.json
	@chmod 0644 $(DIST)/pkgroot/update/etc-sunspec/sunspec.json
	@cp packaging/nameplate.json $(DIST)/pkgroot/update/etc-sunspec/sunspec-nameplate.json
	@chmod 0644 $(DIST)/pkgroot/update/etc-sunspec/sunspec-nameplate.json
	@for f in dropbear dropbearkey dropbearconvert dbclient scp authorized_keys; do \
		[ -f "$(DROPBEAR_DIR)/$$f" ] && cp "$(DROPBEAR_DIR)/$$f" $(DIST)/pkgroot/update/dropbear/ && echo "  +dropbear/$$f" || true; \
	done
	@chmod 0755 $(DIST)/pkgroot/update/dropbear/dropbear* 2>/dev/null || true
	@(cd $(DIST)/pkgroot && tar -cjf ../apsystems-sunspec-$(VERSION)-dropbear.tar.bz2 .)
	@rm -rf $(DIST)/pkgroot
	@ls -lh $(DIST)/apsystems-sunspec-$(VERSION)-dropbear.tar.bz2

# fetch-dropbear — download an armhf dropbear .deb from Debian's wheezy
# security archive, extract the binaries into $(DIST)/dropbear-armv7/.
# Combine with `make package-with-dropbear DROPBEAR_DIR=$(DIST)/dropbear-armv7`.
fetch-dropbear:
	@./packaging/fetch-dropbear.sh $(DIST)/dropbear-armv7

clean:
	rm -rf $(DIST)
