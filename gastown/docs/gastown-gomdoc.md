---
title: gomdoc Gas Town Plugin
---

# gomdoc Gas Town Plugin

A Deacon plugin that automatically runs gomdoc MCP servers for active rigs,
giving all Gas Town agents structured access to project documentation.

## Overview

When a rig is undocked, the Deacon's heartbeat patrol detects it and — if the
rig's repository contains a documentation directory — starts a gomdoc server
on a dedicated port. The MCP endpoint is injected into agent settings so
polecats, crew, witnesses, and the mayor can query docs via keyword search
and section-level reads instead of grepping raw markdown.

## Installation

### Prerequisites

- A running Gas Town workspace
- gomdoc installed on PATH (`gomdoc -version` to verify)

If gomdoc is not installed:

```bash
curl -fsSL https://raw.githubusercontent.com/lacrioque/gomdoc/main/install.sh | sh
```

### Install the plugin

From anywhere inside your Gas Town workspace:

```bash
# Auto-detects GT_ROOT
./install.sh

# Or pass the path explicitly
./install.sh /path/to/gastown

# Or via environment
GT_ROOT=/path/to/gastown ./install.sh
```

The installer copies the plugin and documentation into `$GT_ROOT/plugins/gomdoc/`.

## How It Works

### Rig discovery

On each Deacon heartbeat the plugin:

1. Lists all rigs via `gt rig list`
2. Skips docked and parked rigs
3. Checks each active rig's repo (`$GT_ROOT/<rig>/mayor/rig/`) for a docs
   directory matching any of: `docs/`, `documentation/`, `docus/`, `doc/`, `wiki/`
4. Skips rigs without a matching directory

### Port assignment

Each rig with documentation gets a deterministic port:

- Base port: **7340**
- Rigs sorted alphabetically, port = `7340 + index`

Example with two rigs:

| Rig       | Port |
|-----------|------|
| gomdoc    | 7340 |
| syprasam  | 7341 |

The mapping is persisted in `$GT_ROOT/plugins/gomdoc/state.json`.

### Server lifecycle

- **Start**: If no gomdoc is running on the assigned port, one is spawned:
  `gomdoc -dir <docs_dir> -port <port> -mcp-no-auth -title "<rig> docs"`
- **Verify**: If already running, no action taken
- **Stop**: When a rig is docked/parked or its docs directory disappears,
  the server is killed and removed from state

### MCP injection

The plugin merges an `mcpServers.docs` entry into the `.claude/settings.json`
for each agent type in the rig:

- `$GT_ROOT/<rig>/polecats/.claude/settings.json`
- `$GT_ROOT/<rig>/crew/.claude/settings.json`
- `$GT_ROOT/<rig>/witness/.claude/settings.json`

```json
{
  "mcpServers": {
    "docs": {
      "type": "sse",
      "url": "http://localhost:<port>/mcp/"
    }
  }
}
```

Existing settings and other MCP servers are preserved.

## Agent Usage Guide

Once the plugin is running, agents in that rig have a `docs` MCP server
available with these tools:

| Tool | Purpose |
|------|---------|
| `browse_topics` | See all document headings at a glance |
| `search_documents` | Keyword search with relevance ranking |
| `get_outline` | Table of contents for a single document |
| `read_section` | Read content under a specific heading |
| `list_documents` | List all available documents |
| `read_document` | Read a full document (use sparingly) |

### Recommended workflow

1. **Discover** — `browse_topics` to see what's available
2. **Search** — `search_documents` with keywords
3. **Navigate** — `get_outline` on a promising document
4. **Read** — `read_section` for the part you need

### Rules

- **Always use gomdoc MCP** instead of grepping or reading raw markdown from
  docs directories
- **Search before reading** — call `browse_topics` or `search_documents` first
- **Prefer `read_section`** over `read_document` to save context tokens
- **Use keywords**, not natural language sentences, in search queries

## Files

After installation, the plugin lives at:

```
$GT_ROOT/plugins/gomdoc/
├── plugin.md          # Deacon plugin definition (TOML frontmatter + instructions)
├── state.json         # Runtime state (port mappings, PIDs, last run)
└── docs/
    └── gastown-gomdoc.md   # This documentation
```

## Troubleshooting

### gomdoc not starting

Check that gomdoc is on PATH:

```bash
which gomdoc
gomdoc -version
```

### Port conflict

If another process uses port 7340+, the `curl` health check will pass but MCP
may not work. Check what's listening:

```bash
ss -tlnp | grep 734
```

Kill the conflicting process or adjust the base port in the plugin.

### MCP not showing in agent tools

1. Verify the server is running: `curl -sf http://localhost:<port>/mcp/`
2. Check the settings file has the `mcpServers.docs` entry
3. The agent needs to restart its session to pick up new MCP servers

### Stale state

If `state.json` references a dead PID, the next heartbeat will detect the
server is down and restart it. To force a clean slate:

```bash
echo '{"rigs":{},"last_run":""}' > $GT_ROOT/plugins/gomdoc/state.json
```
