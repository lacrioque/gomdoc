// Package search provides an in-memory full-text search index for markdown files.
// It supports substring search, keyword search with relevance scoring,
// and headline indexing for document structure navigation.
package search

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"gomdoc/renderer"
	"gomdoc/scanner"
)

// Result represents a single search match.
type Result struct {
	// Title is the document title (from frontmatter or filename).
	Title string `json:"title"`
	// Path is the URL path to the document.
	Path string `json:"path"`
	// Snippet is a text excerpt around the match.
	Snippet string `json:"snippet"`
	// Score indicates relevance (higher is better). Only set by keyword search.
	Score float64 `json:"score,omitempty"`
}

// Heading represents a parsed markdown heading within a document.
type Heading struct {
	// Level is the heading depth (1-6).
	Level int `json:"level"`
	// Text is the heading content without the # prefix.
	Text string `json:"text"`
	// Line is the 1-based line number in the source file.
	Line int `json:"line"`
}

// Metadata holds frontmatter fields for a document.
type Metadata struct {
	// Author is the document author.
	Author string `json:"author,omitempty"`
	// Status is the document status (e.g. draft, published, deprecated).
	Status string `json:"status,omitempty"`
	// Date is the document date.
	Date string `json:"date,omitempty"`
	// Tags are the document tags for filtering.
	Tags []string `json:"tags,omitempty"`
	// Category is the document category.
	Category string `json:"category,omitempty"`
	// Version is the document version.
	Version string `json:"version,omitempty"`
	// Reviewers lists the document reviewers.
	Reviewers []string `json:"reviewers,omitempty"`
}

// DocumentOutline describes a document's structure via its headings.
type DocumentOutline struct {
	// Title is the document title.
	Title string `json:"title"`
	// Path is the URL path.
	Path string `json:"path"`
	// Headings is the ordered list of headings in the document.
	Headings []Heading `json:"headings"`
	// Meta holds the document's frontmatter metadata.
	Meta Metadata `json:"meta,omitempty"`
}

// Section holds the content under a specific heading.
type Section struct {
	// Heading is the matched heading text.
	Heading string `json:"heading"`
	// Level is the heading depth.
	Level int `json:"level"`
	// Content is the raw markdown text of the section body.
	Content string `json:"content"`
}

// document stores the indexed content of a single markdown file.
type document struct {
	title    string
	path     string
	content  string         // lowercased plain text for searching
	raw      string         // original text for snippet extraction
	headings []Heading       // parsed headings with line numbers
	keywords map[string]int // word frequency map for keyword search
	meta     Metadata       // frontmatter metadata
}

// Index holds the in-memory search index.
type Index struct {
	mu   sync.RWMutex
	docs []document
}

// NewIndex creates an empty search index.
func NewIndex() *Index {
	return &Index{}
}

// Build scans the base directory and indexes all markdown files.
func (idx *Index) Build(baseDir string) error {
	entries, err := scanner.ScanDirectory(baseDir)
	if err != nil {
		return err
	}

	var docs []document
	for _, entry := range entries {
		doc, err := indexFile(baseDir, entry)
		if err != nil {
			continue // skip unreadable files
		}
		docs = append(docs, doc)
	}

	idx.mu.Lock()
	idx.docs = docs
	idx.mu.Unlock()

	return nil
}

// Search finds documents matching the full query string and returns up to maxResults results.
func (idx *Index) Search(query string, maxResults int) []Result {
	if query == "" {
		return nil
	}

	lowerQuery := strings.ToLower(query)

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []Result
	for _, doc := range idx.docs {
		pos := strings.Index(doc.content, lowerQuery)
		if pos == -1 {
			continue
		}

		snippet := extractSnippet(doc.raw, pos, utf8.RuneCountInString(query))
		results = append(results, Result{
			Title:   doc.title,
			Path:    doc.path,
			Snippet: snippet,
		})

		if len(results) >= maxResults {
			break
		}
	}

	return results
}

// SearchKeywords finds documents matching any of the given keywords,
// ranked by a relevance score based on keyword frequency and match count.
func (idx *Index) SearchKeywords(query string, maxResults int) []Result {
	if query == "" {
		return nil
	}

	keywords := tokenize(query)
	if len(keywords) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	type scored struct {
		doc   document
		score float64
		pos   int // byte position of first match for snippet
	}

	var matches []scored
	for _, doc := range idx.docs {
		score, firstPos := scoreDocument(doc, keywords)
		if score == 0 {
			continue
		}
		matches = append(matches, scored{doc: doc, score: score, pos: firstPos})
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	limit := min(maxResults, len(matches))
	results := make([]Result, limit)
	for i := 0; i < limit; i++ {
		m := matches[i]
		queryLen := len(keywords[0]) // use first keyword for snippet centering
		results[i] = Result{
			Title:   m.doc.title,
			Path:    m.doc.path,
			Snippet: extractSnippet(m.doc.raw, m.pos, queryLen),
			Score:   m.score,
		}
	}

	return results
}

// Outline returns the heading structure for a specific document by path.
func (idx *Index) Outline(docPath string) (DocumentOutline, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	for _, doc := range idx.docs {
		if doc.path == docPath {
			return DocumentOutline{
				Title:    doc.title,
				Path:     doc.path,
				Headings: doc.headings,
				Meta:     doc.meta,
			}, true
		}
	}

	return DocumentOutline{}, false
}

// AllTopics returns headings across all documents, grouped by document.
func (idx *Index) AllTopics() []DocumentOutline {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var topics []DocumentOutline
	for _, doc := range idx.docs {
		if len(doc.headings) == 0 {
			continue
		}
		topics = append(topics, DocumentOutline{
			Title:    doc.title,
			Path:     doc.path,
			Headings: doc.headings,
			Meta:     doc.meta,
		})
	}

	return topics
}

// FindSection returns the content under a heading that matches the given text.
// It searches within the document at docPath using case-insensitive matching.
func (idx *Index) FindSection(docPath, headingQuery string) (Section, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	lowerQuery := strings.ToLower(headingQuery)

	for _, doc := range idx.docs {
		if doc.path != docPath {
			continue
		}
		return extractSection(doc, lowerQuery)
	}

	return Section{}, false
}

// SearchKeywordsWithTags performs keyword search filtered by tags.
// Only documents that have at least one of the specified tags are returned.
// If tags is empty, it behaves identically to SearchKeywords.
func (idx *Index) SearchKeywordsWithTags(query string, tags []string, maxResults int) []Result {
	if len(tags) == 0 {
		return idx.SearchKeywords(query, maxResults)
	}

	if query == "" {
		return idx.filterByTags(tags, maxResults)
	}

	keywords := tokenize(query)
	if len(keywords) == 0 {
		return idx.filterByTags(tags, maxResults)
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	type scored struct {
		doc   document
		score float64
		pos   int
	}

	lowerTags := make([]string, len(tags))
	for i, t := range tags {
		lowerTags[i] = strings.ToLower(t)
	}

	var matches []scored
	for _, doc := range idx.docs {
		if !docHasTag(doc, lowerTags) {
			continue
		}
		score, firstPos := scoreDocument(doc, keywords)
		if score == 0 {
			continue
		}
		matches = append(matches, scored{doc: doc, score: score, pos: firstPos})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	limit := min(maxResults, len(matches))
	results := make([]Result, limit)
	for i := 0; i < limit; i++ {
		m := matches[i]
		queryLen := len(keywords[0])
		results[i] = Result{
			Title:   m.doc.title,
			Path:    m.doc.path,
			Snippet: extractSnippet(m.doc.raw, m.pos, queryLen),
			Score:   m.score,
		}
	}

	return results
}

// filterByTags returns documents matching any of the given tags, without keyword scoring.
func (idx *Index) filterByTags(tags []string, maxResults int) []Result {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	lowerTags := make([]string, len(tags))
	for i, t := range tags {
		lowerTags[i] = strings.ToLower(t)
	}

	var results []Result
	for _, doc := range idx.docs {
		if !docHasTag(doc, lowerTags) {
			continue
		}
		results = append(results, Result{
			Title: doc.title,
			Path:  doc.path,
		})
		if len(results) >= maxResults {
			break
		}
	}

	return results
}

// docHasTag checks whether a document has at least one of the given lowercase tags.
func docHasTag(doc document, lowerTags []string) bool {
	for _, docTag := range doc.meta.Tags {
		lower := strings.ToLower(docTag)
		for _, t := range lowerTags {
			if lower == t {
				return true
			}
		}
	}
	return false
}

// headingPattern matches markdown headings (# through ######).
var headingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// indexFile reads and indexes a single markdown file.
func indexFile(baseDir string, entry scanner.FileEntry) (document, error) {
	filePath := filepath.Join(baseDir, entry.RelPath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return document{}, err
	}

	frontmatter, body := renderer.ParseFrontmatter(content)

	title := frontmatter.Title
	if title == "" {
		title = entry.Name
	}

	urlPath := strings.TrimSuffix(entry.RelPath, filepath.Ext(entry.RelPath))
	urlPath = "/" + filepath.ToSlash(urlPath)

	raw := string(body)
	headings := parseHeadings(raw)
	keywords := buildKeywordMap(raw)

	meta := Metadata{
		Author:    frontmatter.Author,
		Status:    frontmatter.Status,
		Date:      frontmatter.Date,
		Tags:      frontmatter.Tags,
		Category:  frontmatter.Category,
		Version:   frontmatter.Version,
		Reviewers: frontmatter.Reviewers,
	}

	return document{
		title:    title,
		path:     urlPath,
		content:  strings.ToLower(raw),
		raw:      raw,
		headings: headings,
		keywords: keywords,
		meta:     meta,
	}, nil
}

// parseHeadings extracts all markdown headings from the document text.
func parseHeadings(text string) []Heading {
	var headings []Heading
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		match := headingPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		headings = append(headings, Heading{
			Level: len(match[1]),
			Text:  strings.TrimSpace(match[2]),
			Line:  i + 1,
		})
	}

	return headings
}

// tokenize splits a query string into lowercase keyword tokens.
func tokenize(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	// Deduplicate
	seen := make(map[string]bool, len(words))
	var unique []string
	for _, w := range words {
		if len(w) < 2 || seen[w] {
			continue
		}
		seen[w] = true
		unique = append(unique, w)
	}

	return unique
}

// buildKeywordMap creates a word frequency map from document text.
func buildKeywordMap(text string) map[string]int {
	freq := make(map[string]int)
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	for _, w := range words {
		if len(w) < 2 {
			continue
		}
		freq[w]++
	}

	return freq
}

// scoreDocument calculates a relevance score for a document against keywords.
// Returns the score and the byte position of the first keyword match for snippets.
func scoreDocument(doc document, keywords []string) (float64, int) {
	var score float64
	firstPos := -1
	matchedCount := 0

	for _, kw := range keywords {
		count, ok := doc.keywords[kw]
		if !ok {
			continue
		}
		matchedCount++

		// Frequency contributes to score, with diminishing returns
		score += 1.0 + float64(min(count, 10))*0.1

		// Boost for title matches
		if strings.Contains(strings.ToLower(doc.title), kw) {
			score += 3.0
		}

		// Boost for heading matches
		for _, h := range doc.headings {
			if strings.Contains(strings.ToLower(h.Text), kw) {
				score += 2.0
				break
			}
		}

		// Track first occurrence for snippet
		if firstPos == -1 {
			pos := strings.Index(doc.content, kw)
			if pos != -1 {
				firstPos = pos
			}
		}
	}

	if matchedCount == 0 {
		return 0, -1
	}

	// Bonus for matching multiple keywords
	if len(keywords) > 1 {
		score *= float64(matchedCount) / float64(len(keywords))
	}

	return score, firstPos
}

// extractSection finds a section by heading text and returns its content.
func extractSection(doc document, lowerHeading string) (Section, bool) {
	lines := strings.Split(doc.raw, "\n")

	var matchIdx int = -1
	var matchLevel int
	var matchText string

	// Find the matching heading
	for i, line := range lines {
		match := headingPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if strings.Contains(strings.ToLower(match[2]), lowerHeading) {
			matchIdx = i
			matchLevel = len(match[1])
			matchText = strings.TrimSpace(match[2])
			break
		}
	}

	if matchIdx == -1 {
		return Section{}, false
	}

	// Collect content until the next heading of equal or higher level
	var sectionLines []string
	for i := matchIdx + 1; i < len(lines); i++ {
		match := headingPattern.FindStringSubmatch(lines[i])
		if match != nil && len(match[1]) <= matchLevel {
			break
		}
		sectionLines = append(sectionLines, lines[i])
	}

	return Section{
		Heading: matchText,
		Level:   matchLevel,
		Content: strings.TrimSpace(strings.Join(sectionLines, "\n")),
	}, true
}

// extractSnippet returns a text excerpt around the match position.
func extractSnippet(text string, bytePos, queryLen int) string {
	const snippetRadius = 80

	if bytePos < 0 || bytePos >= len(text) {
		if len(text) > snippetRadius*2 {
			return text[:snippetRadius*2] + "..."
		}
		return text
	}

	runes := []rune(text)
	// Convert byte position to rune position
	runePos := utf8.RuneCount([]byte(text[:bytePos]))

	start := max(runePos-snippetRadius, 0)
	end := min(runePos+queryLen+snippetRadius, len(runes))

	snippet := string(runes[start:end])

	// Clean up whitespace
	snippet = strings.Join(strings.Fields(snippet), " ")

	// Add ellipsis if truncated
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet = snippet + "..."
	}

	return snippet
}
