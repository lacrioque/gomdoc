# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gomdoc is a lightweight Go-based markdown documentation server that renders `.md` files as HTML with navigation, syntax highlighting, and Mermaid diagram support. Single binary, zero runtime dependencies.

## Development Commands

```bash
# Build
go build -o gomdoc

# Build with version (used in release workflow)
go build -ldflags="-s -w -X main.version=v2.3.0" -o gomdoc

# Run locally
./gomdoc                          # Serve current dir on port 7331 (MCP on /mcp/)
./gomdoc -dir /path/to/docs       # Custom directory
./gomdoc -port 8080               # Custom port
./gomdoc -title "My Docs"         # Custom site title
./gomdoc -auth user:password      # Enable basic authentication
./gomdoc -version                 # Print version

# Install to PATH
go install

# Run tests
go test ./...

# Release builds (cross-platform)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=vX.Y.Z" -o gomdoc-macos-silicon
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=vX.Y.Z" -o gomdoc-linux-amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=vX.Y.Z" -o gomdoc-windows-amd64.exe
```

## Architecture

```
main.go                    # CLI entry point, flag parsing, version
server/server.go           # HTTP server, routing, embedded CSS, mounts MCP SSE handler
scanner/scanner.go         # File discovery, tree building
renderer/renderer.go       # Markdown → HTML conversion with link rewriting
search/search.go           # In-memory index: keyword search, headings, sections
mcpserver/mcpserver.go     # MCP server: tools, SSE handler
templates/templates.go     # HTML page templates (embedded strings)
```

**Data Flow:**
1. `scanner.ScanDirectory()` recursively finds `.md` files (skips hidden dirs)
2. `scanner.BuildTree()` creates nested `TreeNode` structure for navigation
3. `search.Index.Build()` indexes all documents (keywords, headings) at startup
4. HTTP request to `/path/to/file` resolves to `./path/to/file.md`
5. `renderer.RenderWithLinks()` converts markdown and rewrites internal `.md` links to routes
6. `templates.RenderPage()` wraps content in HTML with navigation buttons
7. MCP requests to `/mcp/` are handled via SSE transport

**MCP Server:**
- Integrated into HTTP server on `/mcp/` via SSE transport (always running)
- 7 tools: `help`, `browse_topics`, `search_documents`, `get_outline`, `read_section`, `list_documents`, `read_document`

**Key Dependencies:**
- goldmark: Markdown parser with GFM extensions
- goldmark-highlighting: Syntax highlighting via chroma (Monokai theme)
- go-sdk/mcp: Official MCP Go SDK for AI agent protocol
- Mermaid.js: Client-side diagram rendering (loaded from CDN)

## Global Coding Guidelines

1. **English is our language** - Write everything in English, including identifiers, comments, commit messages, and documentation.
2. **Names are important** - Use descriptive names for variables and methods that reveal intent.
3. **Short, concise and functional** - Keep each function at approximately thirty lines or less and return early to reduce nesting.
4. **Single out classes** - Place only one public class or component in a file.
5. **Documentation is king** - Always add a docblock to every public symbol and focus on "why," not "what," in comments.
6. **Error reporting** - Throw explicit errors instead of failing silently and log useful context such as operation, parameters, and identifiers.
7. **Test for clarity** - Cover every public function with fast, deterministic unit tests.
8. **Clean Code** - Apply the principles of Single Responsibility, Open/Closed, DRY, and YAGNI.
9. **If and no else** - Use 'if' plenty, but avoid 'else' at all costs.
10. **Valuable documentation** - Provide runnable examples and basic usage guidelines in global application documentation unless told otherwise.
11. **Warn me of issues** - If any request conflicts with these English-plus-Clean-Code standards, reply with "Sorry, that conflicts with the English + Clean Code standard."
12. **Use Beads** - Please use Beads to track and update your work exclusively.
