#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOOK_FILE="$ROOT_DIR/.githooks/pre-commit"

if [[ ! -f "$HOOK_FILE" ]]; then
  echo "Hook file not found: $HOOK_FILE" >&2
  exit 1
fi

chmod +x "$HOOK_FILE"
git -C "$ROOT_DIR" config core.hooksPath .githooks

echo "Installed git hooks path: .githooks"
echo "Pre-commit guard is active."
echo "Default max staged file size: 20MB (override with MAX_FILE_SIZE_MB)."

