package search

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestDir creates a temporary directory with markdown test files.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"hello.md": "# Hello World\nThis is a test document about greetings.\n\n## Introduction\nWelcome to the greeting guide.\n\n## Usage\nUse greetings in daily life.",
		"guide.md": "---\ntitle: User Guide\n---\n# Getting Started\nFollow these steps to begin.\n\n## Installation\nRun the installer command.\n\n## Configuration\nEdit the config file to customize settings.",
		"sub/nested.md": "# Nested File\nThis file lives in a subdirectory.\n\n## Details\nMore details about nesting.",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	return dir
}

func TestBuildAndSearch(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()

	if err := idx.Build(dir); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(idx.docs) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(idx.docs))
	}

	results := idx.Search("greetings", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'greetings', got %d", len(results))
	}
	if results[0].Path != "/hello" {
		t.Errorf("expected path '/hello', got '%s'", results[0].Path)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	results := idx.Search("GETTING STARTED", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive search, got %d", len(results))
	}
	if results[0].Title != "User Guide" {
		t.Errorf("expected title 'User Guide', got '%s'", results[0].Title)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	idx := NewIndex()
	results := idx.Search("", 10)
	if results != nil {
		t.Errorf("expected nil for empty query, got %v", results)
	}
}

func TestSearchMaxResults(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	results := idx.Search("file", 1)
	if len(results) > 1 {
		t.Errorf("expected at most 1 result with maxResults=1, got %d", len(results))
	}
}

func TestSearchNoMatch(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	results := idx.Search("xyznonexistent", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchKeywords(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	results := idx.SearchKeywords("greeting guide", 10)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for keyword search")
	}

	// The hello doc should rank highest because it contains both keywords
	if results[0].Path != "/hello" {
		t.Errorf("expected '/hello' as top result, got '%s'", results[0].Path)
	}
	if results[0].Score <= 0 {
		t.Errorf("expected positive score, got %f", results[0].Score)
	}
}

func TestSearchKeywordsPartialMatch(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	// "installation" appears only in guide.md
	results := idx.SearchKeywords("installation", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'installation', got %d", len(results))
	}
	if results[0].Title != "User Guide" {
		t.Errorf("expected 'User Guide', got '%s'", results[0].Title)
	}
}

func TestSearchKeywordsEmpty(t *testing.T) {
	idx := NewIndex()
	results := idx.SearchKeywords("", 10)
	if results != nil {
		t.Errorf("expected nil for empty keyword query, got %v", results)
	}
}

func TestParseHeadings(t *testing.T) {
	text := "# Title\nSome text.\n## Section A\nMore text.\n### Subsection\nDeep.\n## Section B\n"
	headings := parseHeadings(text)

	if len(headings) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(headings))
	}

	expected := []struct {
		level int
		text  string
		line  int
	}{
		{1, "Title", 1},
		{2, "Section A", 3},
		{3, "Subsection", 5},
		{2, "Section B", 7},
	}

	for i, exp := range expected {
		if headings[i].Level != exp.level {
			t.Errorf("heading %d: expected level %d, got %d", i, exp.level, headings[i].Level)
		}
		if headings[i].Text != exp.text {
			t.Errorf("heading %d: expected text '%s', got '%s'", i, exp.text, headings[i].Text)
		}
		if headings[i].Line != exp.line {
			t.Errorf("heading %d: expected line %d, got %d", i, exp.line, headings[i].Line)
		}
	}
}

func TestOutline(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	outline, found := idx.Outline("/hello")
	if !found {
		t.Fatal("expected to find outline for /hello")
	}

	if outline.Title != "hello" {
		t.Errorf("expected title 'hello', got '%s'", outline.Title)
	}
	if len(outline.Headings) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(outline.Headings))
	}
	if outline.Headings[0].Text != "Hello World" {
		t.Errorf("expected first heading 'Hello World', got '%s'", outline.Headings[0].Text)
	}
}

func TestOutlineNotFound(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	_, found := idx.Outline("/nonexistent")
	if found {
		t.Error("expected not found for nonexistent path")
	}
}

func TestAllTopics(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	topics := idx.AllTopics()
	if len(topics) != 3 {
		t.Fatalf("expected 3 documents with topics, got %d", len(topics))
	}

	// Verify all documents have headings
	for _, doc := range topics {
		if len(doc.Headings) == 0 {
			t.Errorf("expected headings for document '%s'", doc.Title)
		}
	}
}

func TestFindSection(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	section, found := idx.FindSection("/hello", "Introduction")
	if !found {
		t.Fatal("expected to find 'Introduction' section")
	}

	if section.Heading != "Introduction" {
		t.Errorf("expected heading 'Introduction', got '%s'", section.Heading)
	}
	if section.Level != 2 {
		t.Errorf("expected level 2, got %d", section.Level)
	}
	if !containsString(section.Content, "Welcome") {
		t.Errorf("expected content to contain 'Welcome', got '%s'", section.Content)
	}
	// Should not contain content from the next section
	if containsString(section.Content, "daily life") {
		t.Errorf("section content should not include next section, got '%s'", section.Content)
	}
}

func TestFindSectionCaseInsensitive(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	section, found := idx.FindSection("/hello", "usage")
	if !found {
		t.Fatal("expected to find 'usage' section (case-insensitive)")
	}

	if section.Heading != "Usage" {
		t.Errorf("expected heading 'Usage', got '%s'", section.Heading)
	}
}

func TestFindSectionNotFound(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	_, found := idx.FindSection("/hello", "nonexistent heading")
	if found {
		t.Error("expected section not found")
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello, World! hello again")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 unique tokens, got %d: %v", len(tokens), tokens)
	}

	expected := map[string]bool{"hello": true, "world": true, "again": true}
	for _, tok := range tokens {
		if !expected[tok] {
			t.Errorf("unexpected token: %s", tok)
		}
	}
}

func TestTokenizeSkipsShort(t *testing.T) {
	tokens := tokenize("I a am ok go")
	// Only "am", "ok", "go" have length >= 2
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens (skipping single chars), got %d: %v", len(tokens), tokens)
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"hello", "hello", 0},
		{"hello", "helo", 1},
		{"hello", "jello", 1},
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"abc", "", 3},
		{"config", "cnofig", 2}, // transposition-like typo
	}

	for _, tt := range tests {
		got := editDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSearchKeywordsPrefixMatch(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	// "greet" is a prefix of "greeting" and "greetings" in hello.md
	results := idx.SearchKeywords("greet", 10)
	if len(results) == 0 {
		t.Fatal("expected prefix match results for 'greet'")
	}
	if results[0].Path != "/hello" {
		t.Errorf("expected '/hello' as top result for prefix 'greet', got '%s'", results[0].Path)
	}
}

func TestSearchKeywordsFuzzyMatch(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	// "instlalation" is a typo for "installation" (edit distance 2)
	results := idx.SearchKeywords("instlalation", 10)
	if len(results) == 0 {
		t.Fatal("expected fuzzy match results for 'instlalation'")
	}
	if results[0].Title != "User Guide" {
		t.Errorf("expected 'User Guide' for fuzzy match, got '%s'", results[0].Title)
	}
}

func TestSearchKeywordsNoFuzzyForShortWords(t *testing.T) {
	dir := setupTestDir(t)
	idx := NewIndex()
	idx.Build(dir)

	// Short words (< 4 chars) should not trigger fuzzy matching
	results := idx.SearchKeywords("xyz", 10)
	if len(results) != 0 {
		t.Errorf("expected no fuzzy results for short word 'xyz', got %d", len(results))
	}
}

// setupTestDirWithMetadata creates test files with extended frontmatter.
func setupTestDirWithMetadata(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"api.md":      "---\ntitle: API Reference\nauthor: Jane\nstatus: published\ntags: api, rest\ncategory: engineering\n---\n# API Reference\nREST API documentation.\n\n## Endpoints\nGET /users",
		"guide.md":    "---\ntitle: User Guide\nauthor: Bob\nstatus: draft\ntags: guide, onboarding\ncategory: docs\n---\n# User Guide\nGetting started with the platform.",
		"internal.md": "# Internal Notes\nNo frontmatter here.",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	return dir
}

func TestSearchKeywordsWithTags(t *testing.T) {
	dir := setupTestDirWithMetadata(t)
	idx := NewIndex()
	idx.Build(dir)

	// Search with tag filter
	results := idx.SearchKeywordsWithTags("documentation", []string{"api"}, 10)
	for _, r := range results {
		if r.Path != "/api" {
			t.Errorf("expected only /api results with 'api' tag, got '%s'", r.Path)
		}
	}
}

func TestSearchKeywordsWithTagsNoQuery(t *testing.T) {
	dir := setupTestDirWithMetadata(t)
	idx := NewIndex()
	idx.Build(dir)

	// Filter by tag only, no keyword query
	results := idx.SearchKeywordsWithTags("", []string{"guide"}, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result with 'guide' tag, got %d", len(results))
	}
	if results[0].Path != "/guide" {
		t.Errorf("expected /guide, got '%s'", results[0].Path)
	}
}

func TestSearchKeywordsWithTagsNoMatch(t *testing.T) {
	dir := setupTestDirWithMetadata(t)
	idx := NewIndex()
	idx.Build(dir)

	results := idx.SearchKeywordsWithTags("anything", []string{"nonexistent"}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent tag, got %d", len(results))
	}
}

func TestSearchKeywordsWithEmptyTags(t *testing.T) {
	dir := setupTestDirWithMetadata(t)
	idx := NewIndex()
	idx.Build(dir)

	// Empty tags should behave like SearchKeywords
	results := idx.SearchKeywordsWithTags("guide", nil, 10)
	if len(results) == 0 {
		t.Fatal("expected results with empty tags (no filter)")
	}
}

func TestAllTopicsIncludesMetadata(t *testing.T) {
	dir := setupTestDirWithMetadata(t)
	idx := NewIndex()
	idx.Build(dir)

	topics := idx.AllTopics()
	foundMeta := false
	for _, doc := range topics {
		if doc.Title == "API Reference" {
			foundMeta = true
			if doc.Meta.Author != "Jane" {
				t.Errorf("expected author 'Jane', got '%s'", doc.Meta.Author)
			}
			if doc.Meta.Status != "published" {
				t.Errorf("expected status 'published', got '%s'", doc.Meta.Status)
			}
			if len(doc.Meta.Tags) != 2 || doc.Meta.Tags[0] != "api" {
				t.Errorf("expected tags [api, rest], got %v", doc.Meta.Tags)
			}
		}
	}
	if !foundMeta {
		t.Error("expected to find 'API Reference' document in topics")
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
