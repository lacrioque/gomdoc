---
title: gomdoc MCP Server Guide
author: gomdoc
---

# gomdoc MCP Server

gomdoc includes an MCP (Model Context Protocol) server that gives AI coding agents structured access to a markdown documentation directory. Instead of reading raw files, you get keyword search, headline indexing, and section-level access.

## Setup

The MCP server runs automatically alongside the HTTP server on `/mcp/`. Just start gomdoc normally:

```
gomdoc -dir /path/to/docs -port 8080
```

The MCP endpoint is available at `http://localhost:8080/mcp/` using SSE transport.

### Claude Code

Add to your project's `.claude/settings.json`:

```json
{
  "mcpServers": {
    "docs": {
      "type": "sse",
      "url": "http://localhost:8080/mcp/"
    }
  }
}
```

### Other MCP Clients

Any MCP client that supports SSE transport can connect to `http://localhost:<port>/mcp/`.

## Available Tools

### help

Returns a complete usage guide for AI agents, including a ready-to-paste CLAUDE.md / AGENTS.md snippet. Call this first if you are unfamiliar with gomdoc.

Takes no arguments.

### browse_topics

Returns all headings across every document. This is the best starting point when you need to understand what the documentation covers.

Takes no arguments.

Example output:

```
## User Guide [/guide]
  - Getting Started
    - Installation
    - Configuration

## API Reference [/api/reference]
  - Authentication
  - Endpoints
```

### search_documents

Keyword search with relevance ranking. Queries are split into individual words that match independently. Results are scored by keyword frequency, title matches, and heading matches.

Arguments:
- `query` (required) — search terms, e.g. "authentication setup"
- `max_results` (optional) — limit on returned results, default 10

### get_outline

Returns the heading structure of a single document as a table of contents with heading levels and line numbers.

Arguments:
- `path` (required) — document path, e.g. "guide" or "api/reference"

### read_section

Reads the content under a specific heading. Uses case-insensitive partial matching on the heading text. Returns content up to the next heading of equal or higher level.

Arguments:
- `path` (required) — document path
- `heading` (required) — heading text to find, e.g. "installation"

### list_documents

Lists all available documents with their titles and paths.

Arguments:
- `path` (optional) — subdirectory to scope the listing

### read_document

Reads the full markdown content of a document including frontmatter. Use this only when you need the complete text.

Arguments:
- `path` (required) — document path

## Recommended Workflow

When you need to find and understand documentation:

1. **Discover** — call `browse_topics` to see all document headings at a glance
2. **Search** — call `search_documents` with keywords to find relevant documents
3. **Navigate** — call `get_outline` on a document to see its full structure
4. **Read** — call `read_section` to read just the part you need
5. **Full read** — call `read_document` only if you truly need the entire file

This workflow minimizes token usage by avoiding full document reads when a targeted section is enough.

## Scoring

The keyword search scores results using:

- **Frequency** — how often keywords appear in the document (diminishing returns past 10 occurrences)
- **Title boost** (+3.0) — keywords found in the document title
- **Heading boost** (+2.0) — keywords found in any heading
- **Coverage** — matching more of the query keywords ranks higher

Results are sorted by score descending.
