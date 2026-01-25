#!/bin/bash
#
# Build script for Invest Log desktop application
#
# Prerequisites:
#   - Python 3.10+
#   - Rust and Cargo (https://rustup.rs)
#   - PyInstaller: pip install pyinstaller
#   - Tauri CLI: cargo install tauri-cli
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
    
    # Install dependencies if needed
    if ! python3 -m PyInstaller --version &> /dev/null; then
        echo "Installing PyInstaller..."
        pip3 install pyinstaller
    fi

    # Build the sidecar
    python3 -m PyInstaller invest-log-backend.spec --noconfirm
    
    # Copy to Tauri binaries directory
    mkdir -p src-tauri/binaries
    cp "dist/$SIDECAR_NAME" "src-tauri/binaries/"
    
    echo "==> Sidecar built: src-tauri/binaries/$SIDECAR_NAME"
}

# Build Tauri app in development mode
build_dev() {
    echo "==> Starting development build..."
    
    # Ensure sidecar exists
    if [ ! -f "src-tauri/binaries/$SIDECAR_NAME" ]; then
        echo "Sidecar not found, building..."
        build_sidecar
    fi
    
    cd src-tauri
    cargo tauri dev
}

# Build Tauri app for release
build_release() {
    echo "==> Building release..."
    
    # Always rebuild sidecar for release
    build_sidecar
    
    cd src-tauri
    cargo tauri build
    
    echo "==> Release build complete!"
    echo "    Output: src-tauri/target/release/bundle/"
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
