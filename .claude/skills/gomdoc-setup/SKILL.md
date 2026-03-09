---
name: gomdoc-setup
description: Set up gomdoc MCP server for a project. Configures Claude Code settings, generates CLAUDE.md instructions, and optionally installs gomdoc. Use when you want to give an AI agent structured access to a project's markdown documentation.
argument-hint: [docs-directory]
---

# Set up gomdoc MCP Server

Configure the gomdoc MCP server for this project so AI agents get structured
access to the markdown documentation.

Documentation directory: $ARGUMENTS

If no directory was specified, look for common documentation directories
in the current project (docs/, documentation/, doc/, wiki/, or the project
root if it contains markdown files). Ask the user if multiple candidates exist.

## Steps

### 1. Verify gomdoc is installed

Check if `gomdoc` is on PATH:

```
which gomdoc
```

If not found, offer to install it:

```
curl -fsSL https://raw.githubusercontent.com/lacrioque/gomdoc/main/install.sh | sh
```

Verify with:

```
gomdoc -version
```

### 2. Configure MCP server

gomdoc serves MCP on `/mcp/` alongside the HTTP server via SSE transport.

Create or update `.claude/settings.json`:

```json
{
  "mcpServers": {
    "docs": {
      "type": "sse",
      "url": "http://localhost:7331/mcp/"
    }
  }
}
```

If `.claude/settings.json` already exists, merge the `mcpServers` entry
without overwriting other settings.

Tell the user they need to start gomdoc before using the MCP tools:

```
gomdoc -dir <absolute-path-to-docs>
```

### 3. Add agent instructions

Append the following section to the project's CLAUDE.md (create it if it
does not exist). If CLAUDE.md already contains a "Documentation Access"
section, skip this step.

```markdown
### Documentation Access

This project has a gomdoc MCP server connected as "docs" that provides
structured access to project documentation. Use it as follows:

**Finding information:**
- Call `help` for the full usage guide
- Call `browse_topics` to see all available documentation headings
- Call `search_documents` with keywords to find relevant documents
- Call `get_outline` to see a document's table of contents
- Call `read_section` to read a specific section by heading text

**Rules:**
- Always search or browse before reading full documents
- Prefer `read_section` over `read_document` to save context
- Use keyword queries, not natural language sentences
- Check documentation before making assumptions about project conventions
```

### 4. Verify

Check that gomdoc is working:

```
gomdoc -version
```

Report the number of markdown files found in the directory.

### 5. Summary

Tell the user:
- Where the MCP server was configured
- How many documents are available
- Remind them to start gomdoc before using MCP tools
- That they can use `/gomdoc-setup` again if the docs directory changes
- They should restart Claude Code for the MCP server to connect
