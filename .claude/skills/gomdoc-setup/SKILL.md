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

### 2. Configure MCP server

Create or update `.claude/settings.json` to add the gomdoc MCP server.
Use the resolved documentation directory as the `-dir` argument with an
absolute path.

```json
{
  "mcpServers": {
    "docs": {
      "command": "gomdoc",
      "args": ["-mcp", "-dir", "<absolute-path-to-docs>"]
    }
  }
}
```

If `.claude/settings.json` already exists, merge the `mcpServers` entry
without overwriting other settings.

### 3. Add agent instructions

Append the following section to the project's CLAUDE.md (create it if it
does not exist). If CLAUDE.md already contains a "Documentation Access"
section, skip this step.

```markdown
### Documentation Access

This project has a gomdoc MCP server connected as "docs" that provides
structured access to project documentation. Use it as follows:

**Finding information:**
- Call `browse_topics` to see all available documentation headings
- Call `search_documents` with keywords to find relevant documents
- Call `get_outline` to see a document's table of contents
- Call `read_section` to read a specific section by heading text
- Call `help` for the full usage guide

**Rules:**
- Always search or browse before reading full documents
- Prefer `read_section` over `read_document` to save context
- Use keyword queries, not natural language sentences
- Check documentation before making assumptions about project conventions
```

### 4. Verify

Run a quick test to confirm the MCP server starts and can index the docs:

```
echo '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"capabilities":{}}}' | gomdoc -mcp -dir <docs-directory>
```

A clean exit (no errors) means it works. Report the number of markdown
files found in the directory.

### 5. Summary

Tell the user:
- Where the MCP server was configured
- How many documents are available
- That they can use `/gomdoc-setup` again if the docs directory changes
- They should restart Claude Code for the MCP server to connect
