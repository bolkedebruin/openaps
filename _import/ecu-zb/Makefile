BIN := ecu-zb
PKG := ./cmd/ecu-zb
BUILD_DIR := build
ARMV7_BIN := $(BUILD_DIR)/$(BIN)-armv7

ECU_HOST ?=
ECU_DIR ?= /home/applications/ecu-zb

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build armv7 clean test deploy install-init uninstall-init package

all: armv7

build:
	go build -o $(BUILD_DIR)/$(BIN) $(PKG)

armv7:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build -ldflags '-s -w' -trimpath -o $(ARMV7_BIN) $(PKG)
	@echo "built $(ARMV7_BIN) ($$(wc -c <$(ARMV7_BIN)) bytes)"

test:
	go test ./...

deploy: armv7
	@[ -n "$(ECU_HOST)" ] || { echo "ECU_HOST is required (e.g. make deploy ECU_HOST=root@<ip>)"; exit 1; }
	ssh $(ECU_HOST) 'mkdir -p $(ECU_DIR)/run $(ECU_DIR)/log'
	ssh $(ECU_HOST) 'cat > $(ECU_DIR)/$(BIN).new' < $(ARMV7_BIN)
	ssh $(ECU_HOST) 'chmod +x $(ECU_DIR)/$(BIN).new && mv $(ECU_DIR)/$(BIN).new $(ECU_DIR)/$(BIN)'
	@echo "deployed to $(ECU_HOST):$(ECU_DIR)/$(BIN)"

# Install the BusyBox init script that starts ecu-zb at S53 in rcS.d
# (after S50ecu_init brings up /dev/reset and kernel modules for the radio
# hardware-reset bring-up). After installing, reboot the ECU or run
# S53-ecu-zb start manually for it to take effect.
install-init:
	@[ -n "$(ECU_HOST)" ] || { echo "ECU_HOST is required (e.g. make install-init ECU_HOST=root@<ip>)"; exit 1; }
	ssh $(ECU_HOST) 'cat > /etc/rcS.d/S53-ecu-zb' < packaging/S53-ecu-zb
	ssh $(ECU_HOST) 'chmod +x /etc/rcS.d/S53-ecu-zb && ls -la /etc/rcS.d/S53-ecu-zb'
	@echo "init script installed. Reboot the ECU to start ecu-zb at boot."

uninstall-init:
	@[ -n "$(ECU_HOST)" ] || { echo "ECU_HOST is required (e.g. make uninstall-init ECU_HOST=root@<ip>)"; exit 1; }
	ssh $(ECU_HOST) 'rm -f /etc/rcS.d/S53-ecu-zb'
	@echo "init script removed"

clean:
	rm -rf $(BUILD_DIR)

# Build the deployable tarball that the ECU's local upgrade endpoint expects.
#
# Layout produced (matches /index.php/management/exec_upgrade_ecu_app):
#
#   apsystems-ecu-zb-<ver>.tar.bz2
#   ├── update_localweb/
#   │   └── assist                   (shell installer; PHP runs this)
#   └── update/
#       ├── applications/
#       │   └── ecu-zb               (ARMv7 binary)
#       └── etc-rcS-d/
#           └── S53-ecu-zb           (BusyBox rcS.d init script)
#
# Deploy:
#   curl -X POST -F file=@build/apsystems-ecu-zb-<ver>.tar.bz2 \
#        -H 'Expect:' \
#        http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
package: armv7
	@echo "+ packaging $(BIN) $(VERSION)"
	@rm -rf $(BUILD_DIR)/pkgroot
	@mkdir -p $(BUILD_DIR)/pkgroot/update_localweb
	@mkdir -p $(BUILD_DIR)/pkgroot/update/applications
	@mkdir -p $(BUILD_DIR)/pkgroot/update/etc-rcS-d
	@cp $(ARMV7_BIN) $(BUILD_DIR)/pkgroot/update/applications/ecu-zb
	@chmod 0755 $(BUILD_DIR)/pkgroot/update/applications/ecu-zb
	@cp packaging/assist $(BUILD_DIR)/pkgroot/update_localweb/assist
	@chmod 0755 $(BUILD_DIR)/pkgroot/update_localweb/assist
	@cp packaging/S53-ecu-zb $(BUILD_DIR)/pkgroot/update/etc-rcS-d/S53-ecu-zb
	@chmod 0755 $(BUILD_DIR)/pkgroot/update/etc-rcS-d/S53-ecu-zb
	@(cd $(BUILD_DIR)/pkgroot && tar -cjf ../apsystems-ecu-zb-$(VERSION).tar.bz2 .)
	@rm -rf $(BUILD_DIR)/pkgroot
	@ls -lh $(BUILD_DIR)/apsystems-ecu-zb-$(VERSION).tar.bz2
