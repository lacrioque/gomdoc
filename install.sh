#!/bin/sh
set -e

# gomdoc installer
# Usage: curl -fsSL https://raw.githubusercontent.com/lacrioque/gomdoc/main/install.sh | sh
# With version: curl -fsSL ... | sh -s -- v2.0.1
# List versions: curl -fsSL ... | sh -s -- --list

REPO="lacrioque/gomdoc"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-latest}"

usage() {
    echo "gomdoc installer"
    echo ""
    echo "Usage:"
    echo "  curl -fsSL https://raw.githubusercontent.com/$REPO/main/install.sh | sh"
    echo "  curl -fsSL ... | sh -s -- [options]"
    echo ""
    echo "Options:"
    echo "  --help       Show this help message"
    echo "  --list       List available versions"
    echo "  v2.0.1       Install a specific version"
    echo "  latest       Install the latest version (default)"
    echo ""
    echo "Environment:"
    echo "  INSTALL_DIR  Installation directory (default: /usr/local/bin)"
}

list_versions() {
    echo "Available versions:"
    curl -fsSL "https://api.github.com/repos/$REPO/releases" \
        | grep '"tag_name"' \
        | cut -d'"' -f4
}

detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$OS" in
        linux)
            case "$ARCH" in
                x86_64|amd64) PLATFORM="linux-amd64" ;;
                *) echo "Error: unsupported Linux architecture: $ARCH" && exit 1 ;;
            esac
            ;;
        darwin)
            case "$ARCH" in
                arm64)  PLATFORM="macos-silicon" ;;
                x86_64)
                    echo "Note: only Apple Silicon builds are provided, using Rosetta compatibility."
                    PLATFORM="macos-silicon"
                    ;;
                *) echo "Error: unsupported macOS architecture: $ARCH" && exit 1 ;;
            esac
            ;;
        mingw*|msys*|cygwin*)
            PLATFORM="windows-amd64"
            ;;
        *)
            echo "Error: unsupported OS: $OS" && exit 1
            ;;
    esac

    EXT="tar.gz"
    if [ "$PLATFORM" = "windows-amd64" ]; then
        EXT="zip"
    fi

    echo "Detected platform: $PLATFORM"
}

resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
        if [ -z "$VERSION" ]; then
            echo "Error: could not determine latest version" && exit 1
        fi
    fi
    echo "Version: $VERSION"
}

download_and_install() {
    URL="https://github.com/$REPO/releases/download/$VERSION/gomdoc-$PLATFORM.$EXT"
    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR"' EXIT

    echo "Downloading $URL"
    HTTP_CODE="$(curl -fsSL -o "$TMPDIR/gomdoc.$EXT" -w '%{http_code}' "$URL" 2>/dev/null)" || true

    if [ ! -f "$TMPDIR/gomdoc.$EXT" ] || [ "$(wc -c < "$TMPDIR/gomdoc.$EXT")" -lt 1000 ]; then
        echo "Error: download failed. Check that version $VERSION exists."
        echo "Run with --list to see available versions."
        exit 1
    fi

    if [ "$EXT" = "zip" ]; then
        unzip -o "$TMPDIR/gomdoc.$EXT" -d "$TMPDIR" >/dev/null
    else
        tar -xzf "$TMPDIR/gomdoc.$EXT" -C "$TMPDIR"
    fi

    BINARY="$(find "$TMPDIR" -name 'gomdoc*' -not -name '*.tar.gz' -not -name '*.zip' -type f | head -1)"
    if [ -z "$BINARY" ]; then
        echo "Error: binary not found in archive" && exit 1
    fi

    chmod +x "$BINARY"

    mkdir -p "$INSTALL_DIR" 2>/dev/null || true

    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY" "$INSTALL_DIR/gomdoc"
    else
        echo "Installing to $INSTALL_DIR (requires sudo)"
        sudo mkdir -p "$INSTALL_DIR"
        sudo mv "$BINARY" "$INSTALL_DIR/gomdoc"
    fi

    echo ""
    echo "gomdoc $VERSION installed to $INSTALL_DIR/gomdoc"
}

install_skill() {
    SKILL_DIR="$HOME/.claude/skills/gomdoc-setup"
    SKILL_URL="https://raw.githubusercontent.com/$REPO/$VERSION/.claude/skills/gomdoc-setup/SKILL.md"

    echo ""
    echo "Claude Code skill available: /gomdoc-setup"
    echo "  Configures gomdoc MCP for any project with a single command."

    # Check if we can prompt interactively
    if [ ! -t 0 ]; then
        echo ""
        echo "To install the skill globally, run:"
        echo "  mkdir -p $SKILL_DIR && curl -fsSL $SKILL_URL -o $SKILL_DIR/SKILL.md"
        return
    fi

    printf "\nInstall the Claude Code skill globally? [y/N] "
    read -r REPLY < /dev/tty

    case "$REPLY" in
        [yY]|[yY][eE][sS])
            mkdir -p "$SKILL_DIR"
            if curl -fsSL -o "$SKILL_DIR/SKILL.md" "$SKILL_URL"; then
                echo "Skill installed to $SKILL_DIR"
                echo "Use /gomdoc-setup in any project to configure the MCP server."
            else
                echo "Warning: could not download skill file."
                echo "You can install it manually from: $SKILL_URL"
            fi
            ;;
        *)
            echo "Skipped. You can install later with:"
            echo "  mkdir -p $SKILL_DIR && curl -fsSL $SKILL_URL -o $SKILL_DIR/SKILL.md"
            ;;
    esac
}

print_summary() {
    echo ""
    echo "Get started:"
    echo "  gomdoc -dir /path/to/docs        # HTTP server"
    echo "  gomdoc -mcp -dir /path/to/docs   # MCP server for AI agents"
}

# Handle flags
case "$VERSION" in
    --help|-h)
        usage
        exit 0
        ;;
    --list|-l)
        list_versions
        exit 0
        ;;
esac

detect_platform
resolve_version
download_and_install
install_skill
print_summary
