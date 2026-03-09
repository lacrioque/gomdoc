#!/bin/sh
set -e

# gomdoc installer
# Usage: curl -fsSL https://raw.githubusercontent.com/lacrioque/gomdoc/main/install.sh | sh
# With version: curl -fsSL ... | sh -s -- v2.0.0

REPO="lacrioque/gomdoc"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-latest}"

detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$OS" in
        linux)  PLATFORM="linux-amd64" ;;
        darwin) PLATFORM="macos-silicon" ;;
        mingw*|msys*|cygwin*) PLATFORM="windows-amd64" ;;
        *) echo "Unsupported OS: $OS" && exit 1 ;;
    esac

    # Override for x86_64 linux (same binary) and unsupported archs
    if [ "$OS" = "darwin" ] && [ "$ARCH" = "x86_64" ]; then
        echo "Warning: only Apple Silicon builds are provided. Trying Rosetta compatibility."
    fi

    EXT="tar.gz"
    if [ "$PLATFORM" = "windows-amd64" ]; then
        EXT="zip"
    fi
}

resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        VERSION="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | rev | cut -d'/' -f1 | rev)"
        if [ -z "$VERSION" ]; then
            echo "Error: could not determine latest version" && exit 1
        fi
    fi
    echo "Installing gomdoc $VERSION"
}

download_and_install() {
    URL="https://github.com/$REPO/releases/download/$VERSION/gomdoc-$PLATFORM.$EXT"
    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR"' EXIT

    echo "Downloading $URL"
    curl -fsSL -o "$TMPDIR/gomdoc.$EXT" "$URL"

    if [ "$EXT" = "zip" ]; then
        unzip -o "$TMPDIR/gomdoc.$EXT" -d "$TMPDIR" >/dev/null
    else
        tar -xzf "$TMPDIR/gomdoc.$EXT" -C "$TMPDIR"
    fi

    BINARY="$(find "$TMPDIR" -name 'gomdoc*' -not -name '*.tar.gz' -not -name '*.zip' | head -1)"
    if [ -z "$BINARY" ]; then
        echo "Error: binary not found in archive" && exit 1
    fi

    chmod +x "$BINARY"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY" "$INSTALL_DIR/gomdoc"
    else
        echo "Installing to $INSTALL_DIR (requires sudo)"
        sudo mv "$BINARY" "$INSTALL_DIR/gomdoc"
    fi

    echo "gomdoc installed to $INSTALL_DIR/gomdoc"
}

detect_platform
resolve_version
download_and_install
