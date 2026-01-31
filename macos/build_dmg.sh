#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$ROOT_DIR/.." && pwd)"
OUT_DIR="$REPO_DIR/output/macos"
APP_NAME="InvestLog"
APP_TITLE="Invest Log"
APP_BUNDLE_ID="com.investlog.app"

if ! command -v swiftc >/dev/null 2>&1; then
  echo "swiftc not found. Install Xcode Command Line Tools first." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go not found. Install Go first." >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

echo "Building backend (darwin/arm64)..."
BACKEND_BUILD_DIR="$OUT_DIR/backend"
mkdir -p "$BACKEND_BUILD_DIR"
(
  cd "$REPO_DIR/go-backend"
  GOOS=darwin GOARCH=arm64 go build -o "$BACKEND_BUILD_DIR/invest-log-backend" ./cmd/server
)

APP_DIR="$OUT_DIR/${APP_NAME}.app"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"

rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR" "$RESOURCES_DIR"

echo "Copying resources..."
cp -R "$REPO_DIR/static" "$RESOURCES_DIR/static"
cp "$BACKEND_BUILD_DIR/invest-log-backend" "$RESOURCES_DIR/invest-log-backend"
cp "$ROOT_DIR/loading.html" "$RESOURCES_DIR/loading.html"
chmod +x "$RESOURCES_DIR/invest-log-backend"

echo "Compiling macOS app..."
swiftc "$ROOT_DIR/main.swift" \
  -o "$MACOS_DIR/$APP_NAME" \
  -framework AppKit \
  -framework WebKit

echo "Writing Info.plist..."
cat > "$CONTENTS_DIR/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>
  <string>${APP_TITLE}</string>
  <key>CFBundleDisplayName</key>
  <string>${APP_TITLE}</string>
  <key>CFBundleIdentifier</key>
  <string>${APP_BUNDLE_ID}</string>
  <key>CFBundleExecutable</key>
  <string>${APP_NAME}</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0.0</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
  <key>NSAppTransportSecurity</key>
  <dict>
    <key>NSAllowsArbitraryLoads</key>
    <true/>
  </dict>
</dict>
</plist>
EOF

echo "Creating DMG..."
DMG_STAGE="$OUT_DIR/dmg-stage"
rm -rf "$DMG_STAGE"
mkdir -p "$DMG_STAGE"
cp -R "$APP_DIR" "$DMG_STAGE/"
ln -s /Applications "$DMG_STAGE/Applications"

DMG_PATH="$OUT_DIR/${APP_NAME}-macOS-arm64.dmg"
rm -f "$DMG_PATH"
hdiutil create -volname "$APP_TITLE" -srcfolder "$DMG_STAGE" -ov -format UDZO "$DMG_PATH"
rm -rf "$DMG_STAGE"

echo "Done: $DMG_PATH"
