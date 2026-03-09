// Package mcpserver provides an MCP (Model Context Protocol) server
// that exposes gomdoc's documentation content to AI assistants.
// It offers keyword search, headline browsing, section reading,
// and document structure navigation as an external memory for AI agents.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"gomdoc/renderer"
	"gomdoc/scanner"
	"gomdoc/search"
)

// Server wraps the MCP server with access to the documentation directory.
type Server struct {
	baseDir string
	index   *search.Index
	mcp     *mcp.Server
}

// New creates a new MCP server for the given documentation directory.
// The version parameter is displayed in MCP server info responses.
func New(baseDir, version string) *Server {
	s := &Server{
		baseDir: baseDir,
		index:   search.NewIndex(),
	}

	// Silence SDK internal logs (EOF, trailing data) that go to stderr
	// and confuse users doing quick pipe tests.
	s.mcp = mcp.NewServer(&mcp.Implementation{
		Name:    "gomdoc",
		Version: version,
	}, &mcp.ServerOptions{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	s.registerTools()
	return s
}

// Run builds the search index and starts the MCP server on stdio.
// A clean EOF (stdin closed) is treated as a graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	if err := s.index.Build(s.baseDir); err != nil {
		return fmt.Errorf("building search index: %w", err)
	}

	err := s.mcp.Run(ctx, &mcp.StdioTransport{})
	if isEOF(err) {
		return nil
	}
	return err
}

// isEOF checks whether an error is or wraps io.EOF.
// The SDK wraps EOF in "server is closing: EOF", so we check both
// errors.Is and the error string as a fallback.
func isEOF(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	return strings.Contains(err.Error(), "EOF")
}

// --- Argument types ---

// listArgs are the input parameters for the list_documents tool.
type listArgs struct {
	Path string `json:"path,omitempty" jsonschema:"optional subdirectory path to list, leave empty for root"`
}

// readArgs are the input parameters for the read_document tool.
type readArgs struct {
	Path string `json:"path" jsonschema:"document path as shown by list_documents, e.g. guide or sub/nested"`
}

// searchArgs are the input parameters for the search_documents tool.
type searchArgs struct {
	Query      string `json:"query" jsonschema:"text to search for across all documents, supports multi-word keyword queries"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"maximum number of results to return, default 10"`
}

// outlineArgs are the input parameters for the get_outline tool.
type outlineArgs struct {
	Path string `json:"path" jsonschema:"document path to get the outline for, e.g. guide or sub/nested"`
}

// sectionArgs are the input parameters for the read_section tool.
type sectionArgs struct {
	Path    string `json:"path" jsonschema:"document path containing the section"`
	Heading string `json:"heading" jsonschema:"heading text to find, case-insensitive partial match"`
}

// registerTools sets up all MCP tools on the server.
func (s *Server) registerTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "help",
		Description: "Returns a complete guide on how to use the gomdoc MCP server. Includes tool descriptions, recommended workflow, and a ready-to-paste CLAUDE.md / AGENTS.md snippet for configuring AI agents to use this documentation server. Call this first if you are unfamiliar with gomdoc.",
	}, s.handleHelp)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_documents",
		Description: "List all available markdown documents. Returns document titles and paths that can be used with other tools.",
	}, s.handleListDocuments)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "read_document",
		Description: "Read the full content of a markdown document. Returns the raw markdown text and frontmatter metadata. Use get_outline first to understand the structure, then read_section for targeted access.",
	}, s.handleReadDocument)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_documents",
		Description: "Search across all documents using keyword matching with relevance ranking. Multi-word queries match independently and results are scored by keyword frequency, title matches, and heading matches. Use this to find relevant documentation by topic.",
	}, s.handleSearchDocuments)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_outline",
		Description: "Get the heading structure (table of contents) of a document. Returns all headings with their levels and line numbers. Use this to understand document structure before reading specific sections.",
	}, s.handleGetOutline)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "browse_topics",
		Description: "Browse all headings across all documents as a topic index. Returns a structured overview of every document's headings, useful for discovering what documentation covers without reading full content. This is your starting point for exploring the documentation.",
	}, s.handleBrowseTopics)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "read_section",
		Description: "Read a specific section of a document by heading text. Returns only the content under the matched heading, up to the next heading of equal or higher level. Use this for targeted reading instead of loading entire documents.",
	}, s.handleReadSection)
}

// handleHelp returns a usage guide for AI agents, including a ready-to-paste
// configuration snippet for CLAUDE.md or AGENTS.md.
func (s *Server) handleHelp(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	docCount := len(s.index.AllTopics())

	help := fmt.Sprintf(`# gomdoc MCP Server — AI Agent Guide

This MCP server gives you structured access to %d markdown documents.
Instead of reading raw files, use keyword search, headline browsing,
and section-level access to find information efficiently.

## Available Tools

| Tool               | Purpose                                              |
|--------------------|------------------------------------------------------|
| help               | This guide — call it to learn how to use gomdoc      |
| browse_topics      | See all headings across all docs (start here)        |
| search_documents   | Keyword search with relevance ranking                |
| get_outline        | Table of contents for a single document              |
| read_section       | Read content under a specific heading                |
| list_documents     | List all document paths                              |
| read_document      | Read an entire document (use sparingly)              |

## Recommended Workflow

1. browse_topics     — discover what documentation exists
2. search_documents  — find docs by keyword (e.g. "authentication setup")
3. get_outline       — see the structure of a relevant document
4. read_section      — read just the section you need
5. read_document     — only if you need the full file

This approach minimizes token usage. Prefer read_section over read_document.

## Keyword Search Tips

- Multi-word queries match each word independently
- Results are ranked by: keyword frequency, title matches (+3), heading matches (+2)
- Use specific terms: "websocket authentication" not "how do I authenticate websockets"

## CLAUDE.md / AGENTS.md Snippet

Copy the following into your project's CLAUDE.md or AGENTS.md to instruct
AI agents how to use this documentation server:

---

### Documentation Access

This project has a gomdoc MCP server connected as "docs" that provides
structured access to project documentation. Use it as follows:

**Finding information:**
- Call browse_topics to see all available documentation headings
- Call search_documents with keywords to find relevant documents
- Call get_outline to see a document's table of contents
- Call read_section to read a specific section by heading text

**Rules:**
- Always search or browse before reading full documents
- Prefer read_section over read_document to save context
- Use keyword queries, not natural language sentences
- Check documentation before making assumptions about project conventions

---

## MCP Server Configuration

To add this server to a project, include in .claude/settings.json:

{
  "mcpServers": {
    "docs": {
      "command": "gomdoc",
      "args": ["-mcp", "-dir", "/path/to/docs"]
    }
  }
}

Or for Cursor / other MCP clients, use the stdio transport:

  gomdoc -mcp -dir /path/to/docs

The server indexes all .md files at startup and serves them over JSON-RPC.
`, docCount)

	return textResult(help), nil, nil
}

// handleListDocuments returns a list of all available documents.
func (s *Server) handleListDocuments(_ context.Context, _ *mcp.CallToolRequest, args listArgs) (*mcp.CallToolResult, any, error) {
	scanDir := s.baseDir
	if args.Path != "" {
		scanDir = filepath.Join(s.baseDir, filepath.Clean(args.Path))
	}

	entries, err := scanner.ScanDirectory(scanDir)
	if err != nil {
		return nil, nil, fmt.Errorf("scanning directory: %w", err)
	}

	var lines []string
	for _, entry := range entries {
		urlPath := strings.TrimSuffix(entry.RelPath, filepath.Ext(entry.RelPath))
		if args.Path != "" {
			urlPath = args.Path + "/" + urlPath
		}
		lines = append(lines, fmt.Sprintf("- %s (path: %s)", entry.Name, filepath.ToSlash(urlPath)))
	}

	if len(lines) == 0 {
		return textResult("No documents found."), nil, nil
	}

	return textResult(strings.Join(lines, "\n")), nil, nil
}

// handleReadDocument reads and returns a single document's content.
func (s *Server) handleReadDocument(_ context.Context, _ *mcp.CallToolRequest, args readArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return textResult("Error: path is required."), nil, nil
	}

	cleanPath := filepath.Clean(args.Path)
	filePath := filepath.Join(s.baseDir, cleanPath+".md")

	content, err := os.ReadFile(filePath)
	if err != nil {
		// Try uppercase extension
		filePath = filepath.Join(s.baseDir, cleanPath+".MD")
		content, err = os.ReadFile(filePath)
		if err != nil {
			return textResult(fmt.Sprintf("Document not found: %s", args.Path)), nil, nil
		}
	}

	frontmatter, body := renderer.ParseFrontmatter(content)

	var header string
	if frontmatter.Title != "" || frontmatter.Author != "" {
		header = "---\n"
		if frontmatter.Title != "" {
			header += fmt.Sprintf("title: %s\n", frontmatter.Title)
		}
		if frontmatter.Author != "" {
			header += fmt.Sprintf("author: %s\n", frontmatter.Author)
		}
		header += "---\n\n"
	}

	return textResult(header + string(body)), nil, nil
}

// handleSearchDocuments performs keyword search with relevance ranking.
func (s *Server) handleSearchDocuments(_ context.Context, _ *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, any, error) {
	if args.Query == "" {
		return textResult("Error: query is required."), nil, nil
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	results := s.index.SearchKeywords(args.Query, maxResults)
	if len(results) == 0 {
		return textResult("No results found."), nil, nil
	}

	var lines []string
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("## %s (score: %.1f)\nPath: %s\n> %s\n", r.Title, r.Score, r.Path, r.Snippet))
	}

	return textResult(strings.Join(lines, "\n")), nil, nil
}

// handleGetOutline returns the heading structure of a document.
func (s *Server) handleGetOutline(_ context.Context, _ *mcp.CallToolRequest, args outlineArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return textResult("Error: path is required."), nil, nil
	}

	// Normalize path format
	docPath := args.Path
	if !strings.HasPrefix(docPath, "/") {
		docPath = "/" + docPath
	}

	outline, found := s.index.Outline(docPath)
	if !found {
		return textResult(fmt.Sprintf("Document not found: %s", args.Path)), nil, nil
	}

	if len(outline.Headings) == 0 {
		return textResult(fmt.Sprintf("# %s\nNo headings found in this document.", outline.Title)), nil, nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("# %s\n", outline.Title))
	for _, h := range outline.Headings {
		indent := strings.Repeat("  ", h.Level-1)
		lines = append(lines, fmt.Sprintf("%s- %s (line %d)", indent, h.Text, h.Line))
	}

	return textResult(strings.Join(lines, "\n")), nil, nil
}

// handleBrowseTopics returns all headings across all documents.
func (s *Server) handleBrowseTopics(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	topics := s.index.AllTopics()

	if len(topics) == 0 {
		return textResult("No topics found."), nil, nil
	}

	var lines []string
	for _, doc := range topics {
		lines = append(lines, fmt.Sprintf("## %s [%s]", doc.Title, doc.Path))
		for _, h := range doc.Headings {
			indent := strings.Repeat("  ", h.Level-1)
			lines = append(lines, fmt.Sprintf("%s- %s", indent, h.Text))
		}
		lines = append(lines, "")
	}

	return textResult(strings.Join(lines, "\n")), nil, nil
}

// handleReadSection returns the content under a specific heading.
func (s *Server) handleReadSection(_ context.Context, _ *mcp.CallToolRequest, args sectionArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" || args.Heading == "" {
		return textResult("Error: both path and heading are required."), nil, nil
	}

	docPath := args.Path
	if !strings.HasPrefix(docPath, "/") {
		docPath = "/" + docPath
	}

	section, found := s.index.FindSection(docPath, args.Heading)
	if !found {
		return textResult(fmt.Sprintf("Section not found: heading '%s' in document '%s'", args.Heading, args.Path)), nil, nil
	}

	header := fmt.Sprintf("%s %s\n\n", strings.Repeat("#", section.Level), section.Heading)
	return textResult(header + section.Content), nil, nil
}

// textResult creates a simple text content result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}
