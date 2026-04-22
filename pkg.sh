#!/bin/bash

set -eo pipefail

BINARY_PATH=$1
VERSION=$2

APP_ID=io.k8jss.avc-sync
DEVELOPER_ID_APP="Developer ID Application: JSS Technologies, LLC (A3DA4JBHAM)"
DEVELOPER_ID_INSTALLER="Developer ID Installer: JSS Technologies, LLC (A3DA4JBHAM)"
TEAM_ID="A3DA4JBHAM"

ROOT_DIR=pkgbuild
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="avc-sync"
COMP_PKG_PATH="./dist/avc-sync-comp.pkg"
PKG_PATH="./dist/avc-sync_${VERSION}.pkg"

codesign \
  --force \
  --options runtime \
  --identifier "${APP_ID}" \
  --sign "$DEVELOPER_ID_APP" \
  --timestamp \
  "${BINARY_PATH}"

rm -rf "${ROOT_DIR}"
install -d "${ROOT_DIR}${INSTALL_DIR}"
install -m 755 "${BINARY_PATH}" "${ROOT_DIR}${INSTALL_DIR}/${BINARY_NAME}"
install -d "${ROOT_DIR}/Library/LaunchAgents"
install -m 644 "launchagent/${APP_ID}.plist" "${ROOT_DIR}/Library/LaunchAgents/${APP_ID}.plist"

pkgbuild \
  --root "${ROOT_DIR}" \
  --identifier "${APP_ID}" \
  --version "${VERSION}" \
  --scripts pkgscripts \
  "${COMP_PKG_PATH}"

productbuild \
  --sign "${DEVELOPER_ID_INSTALLER}" \
  --package "${COMP_PKG_PATH}" \
  "${PKG_PATH}"

rm -f "${COMP_PKG_PATH}"

if [[ -z $SKIP_NOTARIZE ]]; then
  xcrun notarytool submit "${PKG_PATH}" \
    --apple-id "${NOTARIZE_APPLE_ID}" \
    --password "${NOTARIZE_PASSWORD}" \
    --team-id "${TEAM_ID}" \
    --wait

  xcrun stapler staple "${PKG_PATH}"
fi
