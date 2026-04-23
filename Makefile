BINARY_NAME     := avc-sync
VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

APP_ID              ?= io.k8jss.avc-sync
INSTALL_DIR         ?= /usr/local/bin
ROOT_DIR            ?= pkgbuild
DEVELOPER_ID_APP    ?= Developer ID Application: JSS Technologies, LLC (A3DA4JBHAM)
DEVELOPER_ID_INSTALLER ?= Developer ID Installer: JSS Technologies, LLC (A3DA4JBHAM)
TEAM_ID             ?= A3DA4JBHAM
BINARY_PATH         ?= ./$(BINARY_NAME)
COMP_PKG_PATH       ?= ./dist/$(BINARY_NAME)-comp.pkg
PKG_PATH            ?= ./dist/$(BINARY_NAME)_$(VERSION).pkg

.PHONY: all build test clean sign-binary layout pkg notarize release-pkg

all: build

build:
	CGO_ENABLED=1 go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY_NAME) .

test:
	CGO_ENABLED=1 go test ./...

sign-binary:
	codesign \
		--force \
		--options runtime \
		--identifier "$(APP_ID)" \
		--sign "$(DEVELOPER_ID_APP)" \
		--timestamp \
		"$(BINARY_PATH)"

layout:
	rm -rf "$(ROOT_DIR)"
	install -d "$(ROOT_DIR)$(INSTALL_DIR)"
	install -m 755 "$(BINARY_PATH)" "$(ROOT_DIR)$(INSTALL_DIR)/$(BINARY_NAME)"
	install -d "$(ROOT_DIR)/Library/LaunchAgents"
	install -m 644 "launchagent/$(APP_ID).plist" "$(ROOT_DIR)/Library/LaunchAgents/$(APP_ID).plist"

pkg: sign-binary layout
	mkdir -p dist
	pkgbuild \
		--root "$(ROOT_DIR)" \
		--identifier "$(APP_ID)" \
		--version "$(VERSION)" \
		--scripts pkgscripts \
		"$(COMP_PKG_PATH)"
	productbuild \
		--sign "$(DEVELOPER_ID_INSTALLER)" \
		--package "$(COMP_PKG_PATH)" \
		"$(PKG_PATH)"
	rm -f "$(COMP_PKG_PATH)"

notarize:
ifndef SKIP_NOTARIZE
	xcrun notarytool submit "$(PKG_PATH)" \
		--apple-id "$(NOTARIZE_APPLE_ID)" \
		--password "$(NOTARIZE_PASSWORD)" \
		--team-id "$(TEAM_ID)" \
		--wait
	xcrun stapler staple "$(PKG_PATH)"
endif

release-pkg: pkg notarize

clean:
	rm -rf $(BINARY_NAME) $(ROOT_DIR) dist
