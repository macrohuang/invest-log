#!/bin/bash
#
# Build script for Invest Log desktop application (Electron)
#
# Prerequisites:
#   - Python 3.10+
#   - Node.js + npm
#   - PyInstaller: pip install pyinstaller
#   - Electron Builder: npm install
#
# Usage:
#   ./scripts/build.sh [dev|release|sidecar]
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Detect platform and architecture

detect_platform() {
    case "$(uname -s)" in
        Darwin*)
            PLATFORM="macos"
            case "$(uname -m)" in
                arm64) ARCH="aarch64-apple-darwin" ;;
                x86_64) ARCH="x86_64-apple-darwin" ;;
                *) echo "Unsupported architecture"; exit 1 ;;
            esac
            ;;
        MINGW*|MSYS*|CYGWIN*)
            PLATFORM="windows"
            ARCH="x86_64-pc-windows-msvc"
            ;;
        Linux*)
            PLATFORM="linux"
            ARCH="x86_64-unknown-linux-gnu"
            ;;
        *)
            echo "Unsupported platform"
            exit 1
            ;;
    esac

    SIDECAR_NAME="invest-log-backend-$ARCH"
    if [ "$PLATFORM" = "windows" ]; then
        SIDECAR_NAME="$SIDECAR_NAME.exe"
    fi
}

# Build Python sidecar with PyInstaller

build_sidecar() {
    echo "==> Building Python sidecar..."

    if ! python3 -m PyInstaller --version &> /dev/null; then
        echo "Installing PyInstaller..."
        python3 -m pip install pyinstaller
    fi

    python3 -m PyInstaller invest-log-backend.spec --noconfirm

    if [ ! -f "dist/$SIDECAR_NAME" ]; then
        echo "Sidecar not found at dist/$SIDECAR_NAME"
        exit 1
    fi

    echo "==> Sidecar built: dist/$SIDECAR_NAME"
}

# Ensure Node dependencies

ensure_node_modules() {
    if [ ! -d "node_modules" ]; then
        echo "node_modules not found. Installing npm dependencies..."
        npm install
    fi
}

# Run Electron app in development mode

build_dev() {
    echo "==> Starting Electron development mode..."

    if [ ! -f "dist/$SIDECAR_NAME" ]; then
        echo "Sidecar not found, building..."
        build_sidecar
    fi

    ensure_node_modules
    npm run dev
}

# Build Electron app for release

build_release() {
    echo "==> Building Electron release..."

    build_sidecar
    ensure_node_modules
    npm run dist

    echo "==> Release build complete!"
    echo "    Output: dist/ (electron-builder default)"
}

# Main

detect_platform

case "${1:-release}" in
    dev)
        build_dev
        ;;
    release)
        build_release
        ;;
    sidecar)
        build_sidecar
        ;;
    *)
        echo "Usage: $0 [dev|release|sidecar]"
        exit 1
        ;;
esac
