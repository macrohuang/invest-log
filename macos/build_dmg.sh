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
cp "$ROOT_DIR/AppIcon.icns" "$RESOURCES_DIR/AppIcon.icns"
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
  <key>CFBundleIconFile</key>
  <string>AppIcon</string>
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

RW_DMG_PATH="$OUT_DIR/${APP_NAME}-macOS-arm64.tmp.dmg"
DMG_PATH="$OUT_DIR/${APP_NAME}-macOS-arm64.dmg"
ATTACHED_DEVICE=""
ATTACHED_VOLUME_NAME=""

cleanup() {
  if [[ -n "$ATTACHED_DEVICE" ]]; then
    hdiutil detach "$ATTACHED_DEVICE" -quiet || true
  fi
  rm -rf "$DMG_STAGE"
  rm -f "$RW_DMG_PATH"
}
trap cleanup EXIT

STAGE_SIZE_MB="$(du -sm "$DMG_STAGE" | awk '{print $1}')"
DMG_SIZE_MB=$((STAGE_SIZE_MB + 80))

rm -f "$DMG_PATH" "$RW_DMG_PATH"
hdiutil create -volname "$APP_TITLE" -srcfolder "$DMG_STAGE" -ov -format UDRW -size "${DMG_SIZE_MB}m" "$RW_DMG_PATH"

ATTACH_INFO="$(hdiutil attach "$RW_DMG_PATH" -readwrite -noverify -noautoopen)"
ATTACHED_DEVICE="$(echo "$ATTACH_INFO" | awk '/^\/dev\// { print $1; exit }')"
ATTACHED_VOLUME_PATH="$(echo "$ATTACH_INFO" | awk -F'\t' '/\/Volumes\// { print $NF; exit }')"
ATTACHED_VOLUME_NAME="${ATTACHED_VOLUME_PATH##*/}"
if [[ -z "$ATTACHED_VOLUME_NAME" ]]; then
  ATTACHED_VOLUME_NAME="$APP_TITLE"
fi

if command -v osascript >/dev/null 2>&1; then
  for _ in 1 2 3 4 5; do
    if osascript -e "tell application \"Finder\" to get name of disk \"${ATTACHED_VOLUME_NAME}\"" >/dev/null 2>&1; then
      break
    fi
    sleep 0.5
  done

  if ! osascript <<EOF
tell application "Finder"
  tell disk "${ATTACHED_VOLUME_NAME}"
    open
    set current view of container window to icon view
    set toolbar visible of container window to false
    set statusbar visible of container window to false
    set bounds of container window to {120, 120, 1180, 760}

    set opts to the icon view options of container window
    set arrangement of opts to not arranged
    set icon size of opts to 128
    set text size of opts to 16

    set position of item "${APP_NAME}.app" of container window to {300, 320}
    set position of item "Applications" of container window to {760, 320}

    update without registering applications
    delay 1
    close
  end tell
end tell
EOF
  then
    echo "Warning: failed to apply Finder layout; continue with default layout." >&2
  fi
else
  echo "Warning: osascript not found; continue with default layout." >&2
fi

sync
hdiutil detach "$ATTACHED_DEVICE"
ATTACHED_DEVICE=""

hdiutil convert "$RW_DMG_PATH" -format UDZO -imagekey zlib-level=9 -ov -o "$DMG_PATH"

trap - EXIT
cleanup

echo "Done: $DMG_PATH"
