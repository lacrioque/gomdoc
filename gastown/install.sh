#!/bin/sh
set -e

# gomdoc Gas Town plugin installer
# Installs the gomdoc Deacon plugin into a Gas Town workspace.
#
# Usage:
#   ./install.sh                      # Auto-detect GT_ROOT from cwd
#   ./install.sh /path/to/gastown     # Explicit GT_ROOT
#   GT_ROOT=/path/to/gt ./install.sh  # Via environment

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GT_ROOT="${1:-${GT_ROOT:-}}"

# --- Detect GT_ROOT ---

if [ -z "$GT_ROOT" ]; then
    # Walk up from cwd looking for a Gas Town workspace
    CHECK="$(pwd)"
    while [ "$CHECK" != "/" ]; do
        if [ -f "$CHECK/.beads/routes.jsonl" ] || [ -d "$CHECK/mayor" ]; then
            GT_ROOT="$CHECK"
            break
        fi
        CHECK="$(dirname "$CHECK")"
    done
fi

if [ -z "$GT_ROOT" ]; then
    echo "Error: Could not detect Gas Town workspace."
    echo "Run from inside a GT workspace, pass the path as argument, or set GT_ROOT."
    exit 1
fi

if [ ! -d "$GT_ROOT/plugins" ]; then
    echo "Error: $GT_ROOT/plugins/ does not exist. Is this a Gas Town workspace?"
    exit 1
fi

echo "Gas Town root: $GT_ROOT"

# --- Check gomdoc is installed ---

if ! command -v gomdoc >/dev/null 2>&1; then
    echo ""
    echo "gomdoc is not installed."
    echo "Install it first:"
    echo "  curl -fsSL https://raw.githubusercontent.com/lacrioque/gomdoc/main/install.sh | sh"
    exit 1
fi

GOMDOC_VERSION="$(gomdoc -version 2>/dev/null || echo "unknown")"
echo "gomdoc version: $GOMDOC_VERSION"

# --- Install plugin ---

PLUGIN_DIR="$GT_ROOT/plugins/gomdoc"
mkdir -p "$PLUGIN_DIR"

cp "$SCRIPT_DIR/plugin.md" "$PLUGIN_DIR/plugin.md"
echo "Installed plugin: $PLUGIN_DIR/plugin.md"

# --- Install documentation ---

DOCS_DIR="$PLUGIN_DIR/docs"
mkdir -p "$DOCS_DIR"

if [ -f "$SCRIPT_DIR/docs/gastown-gomdoc.md" ]; then
    cp "$SCRIPT_DIR/docs/gastown-gomdoc.md" "$DOCS_DIR/gastown-gomdoc.md"
    echo "Installed docs:   $DOCS_DIR/gastown-gomdoc.md"
fi

# --- Initialize state.json if missing ---

if [ ! -f "$PLUGIN_DIR/state.json" ]; then
    echo '{"rigs":{},"last_run":""}' > "$PLUGIN_DIR/state.json"
    echo "Created state:    $PLUGIN_DIR/state.json"
fi

# --- Summary ---

echo ""
echo "gomdoc Gas Town plugin installed."
echo ""
echo "What it does:"
echo "  - On each Deacon heartbeat, discovers active rigs with docs"
echo "  - Starts a gomdoc MCP server per rig (ports 7340+)"
echo "  - Injects MCP config into polecat/crew/witness settings"
echo "  - Cleans up servers when rigs are docked"
echo ""
echo "The plugin will run on the next Deacon patrol cycle."
echo "To test immediately: start gomdoc manually for a rig:"
echo "  gomdoc -dir \$GT_ROOT/<rig>/mayor/rig/docs -port 7340 -mcp-no-auth &"
