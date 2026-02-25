#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <path-to-dmg>" >&2
  exit 1
fi

DMG_PATH="$1"

if [[ ! -f "$DMG_PATH" ]]; then
  echo "dmg not found: $DMG_PATH" >&2
  exit 1
fi

MOUNT_DIR="$(mktemp -d /tmp/investlog-dmg-check.XXXXXX)"
ATTACH_INFO=""

cleanup() {
  if [[ -n "$ATTACH_INFO" ]]; then
    local device
    device="$(echo "$ATTACH_INFO" | awk '/^\/dev\// { print $1; exit }')"
    if [[ -n "$device" ]]; then
      hdiutil detach "$device" -quiet || true
    fi
  fi
  rm -rf "$MOUNT_DIR"
}
trap cleanup EXIT

ATTACH_INFO="$(hdiutil attach "$DMG_PATH" -readonly -nobrowse -mountpoint "$MOUNT_DIR")"

[[ -d "$MOUNT_DIR/InvestLog.app" ]] || {
  echo "missing InvestLog.app in dmg" >&2
  exit 1
}

[[ -L "$MOUNT_DIR/Applications" ]] || {
  echo "missing Applications symlink in dmg" >&2
  exit 1
}

[[ -f "$MOUNT_DIR/.DS_Store" ]] || {
  echo "missing .DS_Store (Finder layout metadata)" >&2
  exit 1
}

echo "OK: DMG layout checks passed"
