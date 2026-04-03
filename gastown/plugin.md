+++
name = "gomdoc"
parallel = false
gate = { type = "event", value = "heartbeat" }

[execution]
timeout = "2m"
+++

# gomdoc — Documentation MCP Server for Rigs

Start a gomdoc MCP server for each active (undocked) rig whose repository
contains a documentation directory. This gives polecats, witnesses, and the
mayor structured keyword search and section-level access to project docs
instead of grepping through raw markdown files.

## Execution

### Step 1: Discover active rigs with documentation

For each rig returned by `gt rig list`:

1. Skip rigs that are **docked** or **parked** (`gt rig status <rig>` — check Status line).
2. Locate the rig's repo checkout at `$GT_ROOT/<rig>/mayor/rig/`.
3. Check whether **any** of these directories exist inside the checkout:
   - `docs/`
   - `documentation/`
   - `docus/`
   - `doc/`
   - `wiki/`

   If **none** exist, skip this rig — it has no documentation to serve.

### Step 2: Assign a port

Each rig gets a deterministic port so MCP URLs stay stable across restarts:

| Port  | Rig        |
|-------|------------|
| 7340  | 1st rig (alphabetical) |
| 7341  | 2nd rig    |
| 7342  | 3rd rig    |
| …     | …          |

Base port is **7340**. Sort active-with-docs rigs alphabetically, assign
`7340 + index`. Store the mapping in `$GT_ROOT/plugins/gomdoc/state.json`
so other steps can look it up:

```json
{
  "rigs": {
    "syprasam": { "port": 7340, "docs_dir": "/home/markus/projects/gt/syprasam/mayor/rig/docs", "pid": 12345 }
  },
  "last_run": "2026-04-03T09:30:00Z"
}
```

### Step 3: Start or verify gomdoc processes

For each rig with docs:

1. Check if a gomdoc process is already running on the assigned port:
   ```bash
   curl -sf http://localhost:<port>/mcp/ -o /dev/null && echo "running" || echo "stopped"
   ```
2. If **stopped**, start gomdoc in the background:
   ```bash
   gomdoc -dir <docs_dir> -port <port> -mcp-no-auth -title "<rig> docs" &
   ```
   Record the PID in `state.json`.
3. If **running**, no action needed.

### Step 4: Inject MCP config into agent settings

For each rig with a running gomdoc server, ensure the `.claude/settings.json`
files for that rig's agents contain the MCP server entry.

Merge into the existing `mcpServers` object (do not overwrite other servers):

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

Inject into these locations:
- `$GT_ROOT/<rig>/polecats/.claude/settings.json` (polecat agents)
- `$GT_ROOT/<rig>/crew/.claude/settings.json` (crew agents)
- `$GT_ROOT/<rig>/witness/.claude/settings.json` (witness agent)

### Step 5: Clean up stale servers

Check `state.json` for rigs that are no longer active (docked/parked) or whose
docs directory no longer exists. Kill their gomdoc process (`kill <pid>`) and
remove them from `state.json`. Also remove the `mcpServers.docs` entry from
the rig's settings files.

## Rules for Agents Using gomdoc MCP

Polecats, witnesses, and crew agents in rigs with gomdoc should follow these rules
(enforced via CLAUDE.md in each rig):

1. **Use gomdoc MCP tools instead of grepping markdown files.**
   When you need information from project documentation, use the `docs` MCP server:
   - `browse_topics` to see all available headings
   - `search_documents` with keywords to find relevant docs
   - `read_section` to read a specific section by heading
   - Avoid `read_document` for full files — prefer targeted `read_section`

2. **Never grep or read raw markdown from the docs/ directory** when the `docs`
   MCP server is available. The MCP server provides keyword-ranked search and
   structured section access that is faster and uses less context.

3. **Search before reading.** Always call `browse_topics` or `search_documents`
   before pulling full document content.
