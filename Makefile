BINARY_NAME     := avc-sync
APP_ID          := io.k8jss.avc-sync
INSTALL_DIR     := /usr/local/bin
LAUNCH_AGENT    := /Library/LaunchAgents/$(APP_ID).plist
VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

BUILD_DIR       := build
ROOT_DIR        := $(BUILD_DIR)/root
PKG_NAME        := $(BINARY_NAME)-$(VERSION).pkg

# Signing — override via environment or make invocation
DEVELOPER_ID_APP       ?= Developer ID Application: JSS Technologies, LLC (A3DA4JBHAM)
DEVELOPER_ID_INSTALLER ?= Developer ID Installer: JSS Technologies, LLC (A3DA4JBHAM)
NOTARIZE_APPLE_ID      ?=
NOTARIZE_PASSWORD      ?=
NOTARIZE_TEAM_ID       ?=

.PHONY: all build build-arm64 build-amd64 universal sign-binary layout package sign-pkg notarize staple release clean

all: build

## Local build for the current architecture (unsigned, for development)
build:
	CGO_ENABLED=1 go build -ldflags="-X main.version=$(VERSION)" \
		-o $(BINARY_NAME) ./cmd/$(BINARY_NAME)

## Build universal binary (arm64 + amd64) for distribution
build-arm64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
		-ldflags="-X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-arm64 ./cmd/$(BINARY_NAME)

build-amd64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build \
		-ldflags="-X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-amd64 ./cmd/$(BINARY_NAME)

universal: build-arm64 build-amd64
	lipo -create -output $(BUILD_DIR)/$(BINARY_NAME) \
		$(BUILD_DIR)/$(BINARY_NAME)-arm64 \
		$(BUILD_DIR)/$(BINARY_NAME)-amd64
	rm -f $(BUILD_DIR)/$(BINARY_NAME)-arm64 $(BUILD_DIR)/$(BINARY_NAME)-amd64

## Sign the binary with Hardened Runtime (required for notarization)
sign-binary: universal
	codesign --force --options runtime \
		--sign "$(DEVELOPER_ID_APP)" \
		$(BUILD_DIR)/$(BINARY_NAME)
	codesign --verify --verbose $(BUILD_DIR)/$(BINARY_NAME)

## Assemble the PKG root payload directory structure
layout: sign-binary
	rm -rf $(ROOT_DIR)
	install -d "$(ROOT_DIR)$(INSTALL_DIR)"
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) "$(ROOT_DIR)$(INSTALL_DIR)/$(BINARY_NAME)"
	install -d "$(ROOT_DIR)/Library/LaunchAgents"
	install -m 644 launchagent/$(APP_ID).plist "$(ROOT_DIR)/Library/LaunchAgents/$(APP_ID).plist"

## Build an unsigned component package, then wrap it in a signed distribution package
package: layout
	pkgbuild \
		--root $(ROOT_DIR) \
		--identifier $(APP_ID) \
		--version $(VERSION) \
		--scripts pkgscripts \
		$(BUILD_DIR)/$(BINARY_NAME)-component.pkg

## Sign the pkg with the Developer ID Installer certificate
sign-pkg: package
	productbuild \
		--sign "$(DEVELOPER_ID_INSTALLER)" \
		--package $(BUILD_DIR)/$(BINARY_NAME)-component.pkg \
		$(BUILD_DIR)/$(PKG_NAME)
	rm -f $(BUILD_DIR)/$(BINARY_NAME)-component.pkg
	@echo "Signed package: $(BUILD_DIR)/$(PKG_NAME)"

## Submit to Apple for notarization
notarize: sign-pkg
	xcrun notarytool submit $(BUILD_DIR)/$(PKG_NAME) \
		--apple-id "$(NOTARIZE_APPLE_ID)" \
		--password "$(NOTARIZE_PASSWORD)" \
		--team-id "$(NOTARIZE_TEAM_ID)" \
		--wait

## Staple the notarization ticket to the pkg
staple: notarize
	xcrun stapler staple $(BUILD_DIR)/$(PKG_NAME)
	@echo "Release package ready: $(BUILD_DIR)/$(PKG_NAME)"

## Full release pipeline: build → sign → package → notarize → staple
release: staple

## Run tests
test:
	CGO_ENABLED=1 go test ./...

clean:
	rm -rf $(BUILD_DIR) $(BINARY_NAME)
