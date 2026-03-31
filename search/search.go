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

// DocumentOutline describes a document's structure via its headings.
type DocumentOutline struct {
	// Title is the document title.
	Title string `json:"title"`
	// Path is the URL path.
	Path string `json:"path"`
	// Headings is the ordered list of headings in the document.
	Headings []Heading `json:"headings"`
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
	content  string    // lowercased plain text for searching
	raw      string    // original text for snippet extraction
	headings []Heading // parsed headings with line numbers
	keywords map[string]int // word frequency map for keyword search
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

	return document{
		title:    title,
		path:     urlPath,
		content:  strings.ToLower(raw),
		raw:      raw,
		headings: headings,
		keywords: keywords,
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
// Supports exact, prefix, and fuzzy (edit distance <= 2) matching with decreasing weight.
func scoreDocument(doc document, keywords []string) (float64, int) {
	var score float64
	firstPos := -1
	matchedCount := 0

	for _, kw := range keywords {
		kwScore, pos := scoreKeyword(doc, kw)
		if kwScore == 0 {
			continue
		}
		matchedCount++
		score += kwScore

		if firstPos == -1 && pos != -1 {
			firstPos = pos
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

// scoreKeyword scores a single keyword against a document.
// Tries exact match first, then prefix, then fuzzy. Returns score and byte position.
func scoreKeyword(doc document, kw string) (float64, int) {
	// Exact match
	if count, ok := doc.keywords[kw]; ok {
		s := 1.0 + float64(min(count, 10))*0.1
		s += titleHeadingBoost(doc, kw)
		pos := strings.Index(doc.content, kw)
		return s, pos
	}

	// Prefix match: keyword is a prefix of a document word (min 3 chars)
	if len(kw) >= 3 {
		if s, pos := prefixMatch(doc, kw); s > 0 {
			return s, pos
		}
	}

	// Fuzzy match: edit distance for typo tolerance (min 4 chars)
	if len(kw) >= 4 {
		if s, pos := fuzzyMatch(doc, kw); s > 0 {
			return s, pos
		}
	}

	return 0, -1
}

// prefixMatch finds document words that start with the keyword prefix.
// Returns a reduced score (0.6x) since prefix matches are less precise.
func prefixMatch(doc document, prefix string) (float64, int) {
	var totalCount int
	for word, count := range doc.keywords {
		if strings.HasPrefix(word, prefix) {
			totalCount += count
		}
	}

	if totalCount == 0 {
		return 0, -1
	}

	score := 0.6 * (1.0 + float64(min(totalCount, 10))*0.1)
	score += 0.6 * titleHeadingBoost(doc, prefix)
	pos := strings.Index(doc.content, prefix)
	return score, pos
}

// fuzzyMatch finds document words within edit distance 1-2 of the keyword.
// Returns a reduced score (0.3x) since fuzzy matches may be false positives.
func fuzzyMatch(doc document, kw string) (float64, int) {
	bestCount := 0
	bestWord := ""

	for word, count := range doc.keywords {
		dist := editDistance(kw, word)
		if dist == 0 || dist > maxEditDistance(kw) {
			continue
		}
		if count > bestCount {
			bestCount = count
			bestWord = word
		}
	}

	if bestCount == 0 {
		return 0, -1
	}

	score := 0.3 * (1.0 + float64(min(bestCount, 10))*0.1)
	pos := strings.Index(doc.content, bestWord)
	return score, pos
}

// maxEditDistance returns the allowed edit distance based on keyword length.
func maxEditDistance(kw string) int {
	if len(kw) <= 5 {
		return 1
	}
	return 2
}

// titleHeadingBoost returns bonus score for keywords found in title or headings.
func titleHeadingBoost(doc document, kw string) float64 {
	var boost float64
	if strings.Contains(strings.ToLower(doc.title), kw) {
		boost += 3.0
	}
	for _, h := range doc.headings {
		if strings.Contains(strings.ToLower(h.Text), kw) {
			boost += 2.0
			break
		}
	}
	return boost
}

// editDistance computes the Levenshtein distance between two strings.
// Returns early if distance exceeds maxDist (optimization for large vocabularies).
func editDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)

	// Quick rejection: length difference alone exceeds max possible useful distance
	if abs(la-lb) > 2 {
		return 3
	}

	// Use single-row DP for space efficiency
	prev := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = curr
	}

	return prev[lb]
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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
