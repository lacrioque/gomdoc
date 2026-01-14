# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gomdoc is a lightweight Go-based markdown documentation server that renders `.md` files as HTML with navigation, syntax highlighting, and Mermaid diagram support. Single binary, zero runtime dependencies.

## Development Commands

```bash
# Build
go build -o gomdoc

# Run locally
./gomdoc                          # Serve current dir on port 7331
./gomdoc -dir /path/to/docs       # Custom directory
./gomdoc -port 8080               # Custom port
./gomdoc -title "My Docs"         # Custom site title
./gomdoc -auth user:password      # Enable basic authentication

# Install to PATH
go install

# Release builds (cross-platform)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o gomdoc-macos-silicon
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o gomdoc-linux-amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o gomdoc-windows-amd64.exe
```

## Architecture

```
main.go                    # CLI entry point, flag parsing
server/server.go           # HTTP server, routing, embedded CSS
scanner/scanner.go         # File discovery, tree building
renderer/renderer.go       # Markdown → HTML conversion with link rewriting
templates/templates.go     # HTML page templates (embedded strings)
```

**Data Flow:**
1. `scanner.ScanDirectory()` recursively finds `.md` files (skips hidden dirs)
2. `scanner.BuildTree()` creates nested `TreeNode` structure for navigation
3. HTTP request to `/path/to/file` resolves to `./path/to/file.md`
4. `renderer.RenderWithLinks()` converts markdown and rewrites internal `.md` links to routes
5. `templates.RenderPage()` wraps content in HTML with navigation buttons

**Key Dependencies:**
- goldmark: Markdown parser with GFM extensions
- goldmark-highlighting: Syntax highlighting via chroma (Monokai theme)
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
