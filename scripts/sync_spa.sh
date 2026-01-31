#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_DIR="$ROOT_DIR/static"
DEST_DIR="$ROOT_DIR/ios/App/App/public"

if [[ ! -d "$SRC_DIR" ]]; then
  echo "Source static/ not found: $SRC_DIR" >&2
  exit 1
fi

if [[ ! -d "$DEST_DIR" ]]; then
  echo "Destination not found: $DEST_DIR" >&2
  exit 1
fi

rsync -a --delete \
  --exclude '.DS_Store' \
  "$SRC_DIR/" "$DEST_DIR/"

echo "Synced $SRC_DIR -> $DEST_DIR"
