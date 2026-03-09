package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"gomdoc/search"
)

// setupTestDir creates a temporary directory with markdown test files.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"hello.md":      "# Hello World\nThis is a test document about greetings.\n\n## Introduction\nWelcome to the greeting guide.\n\n## Usage\nUse greetings in daily life.",
		"guide.md":      "---\ntitle: User Guide\nauthor: Test Author\n---\n# Getting Started\nFollow these steps to begin.\n\n## Installation\nRun the installer command.\n\n## Configuration\nEdit the config file.",
		"sub/nested.md": "# Nested File\nThis file lives in a subdirectory.\n\n## Details\nMore details about nesting.",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	return dir
}

// newTestServer creates a server with a built index for testing.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := setupTestDir(t)
	idx := search.NewIndex()
	idx.Build(dir)

	return &Server{
		baseDir: dir,
		index:   idx,
	}
}

func TestHandleListDocuments(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleListDocuments(ctx, nil, listArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", text)
	}
	if !strings.Contains(text, "guide") {
		t.Errorf("expected 'guide' in output, got: %s", text)
	}
}

func TestHandleListDocumentsSubdir(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleListDocuments(ctx, nil, listArgs{Path: "sub"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "nested") {
		t.Errorf("expected 'nested' in output, got: %s", text)
	}
}

func TestHandleReadDocument(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleReadDocument(ctx, nil, readArgs{Path: "guide"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "title: User Guide") {
		t.Errorf("expected frontmatter title in output, got: %s", text)
	}
	if !strings.Contains(text, "Getting Started") {
		t.Errorf("expected body content in output, got: %s", text)
	}
}

func TestHandleReadDocumentNotFound(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleReadDocument(ctx, nil, readArgs{Path: "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' message, got: %s", text)
	}
}

func TestHandleSearchDocuments(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleSearchDocuments(ctx, nil, searchArgs{Query: "greeting guide"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "hello") {
		t.Errorf("expected 'hello' in results, got: %s", text)
	}
	if !strings.Contains(text, "score:") {
		t.Errorf("expected score in results, got: %s", text)
	}
}

func TestHandleSearchDocumentsEmpty(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleSearchDocuments(ctx, nil, searchArgs{Query: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "required") {
		t.Errorf("expected error message for empty query, got: %s", text)
	}
}

func TestHandleGetOutline(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleGetOutline(ctx, nil, outlineArgs{Path: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected 'Hello World' heading in outline, got: %s", text)
	}
	if !strings.Contains(text, "Introduction") {
		t.Errorf("expected 'Introduction' heading in outline, got: %s", text)
	}
	if !strings.Contains(text, "Usage") {
		t.Errorf("expected 'Usage' heading in outline, got: %s", text)
	}
}

func TestHandleGetOutlineNotFound(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleGetOutline(ctx, nil, outlineArgs{Path: "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' message, got: %s", text)
	}
}

func TestHandleBrowseTopics(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleBrowseTopics(ctx, nil, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	// Should contain headings from all documents
	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected 'Hello World' in topics, got: %s", text)
	}
	if !strings.Contains(text, "Getting Started") {
		t.Errorf("expected 'Getting Started' in topics, got: %s", text)
	}
	if !strings.Contains(text, "Nested File") {
		t.Errorf("expected 'Nested File' in topics, got: %s", text)
	}
}

func TestHandleReadSection(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleReadSection(ctx, nil, sectionArgs{Path: "hello", Heading: "Introduction"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "## Introduction") {
		t.Errorf("expected section header, got: %s", text)
	}
	if !strings.Contains(text, "Welcome") {
		t.Errorf("expected section content, got: %s", text)
	}
}

func TestHandleReadSectionNotFound(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleReadSection(ctx, nil, sectionArgs{Path: "hello", Heading: "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' message, got: %s", text)
	}
}

func TestHandleHelp(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleHelp(ctx, nil, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "AI Agent Guide") {
		t.Errorf("expected guide header, got: %s", text[:100])
	}
	if !strings.Contains(text, "browse_topics") {
		t.Errorf("expected tool reference in help text")
	}
	if !strings.Contains(text, "CLAUDE.md") {
		t.Errorf("expected CLAUDE.md snippet in help text")
	}
	if !strings.Contains(text, "mcpServers") {
		t.Errorf("expected MCP config example in help text")
	}
}

func TestHandleReadSectionMissingArgs(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	result, _, err := s.handleReadSection(ctx, nil, sectionArgs{Path: "", Heading: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "required") {
		t.Errorf("expected 'required' error, got: %s", text)
	}
}
