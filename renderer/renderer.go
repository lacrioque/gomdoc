// Package renderer provides markdown to HTML conversion with link rewriting.
package renderer

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Frontmatter holds metadata parsed from YAML frontmatter.
type Frontmatter struct {
	Title     string
	Author    string
	Status    string
	Date      string
	Tags      []string
	Category  string
	Version   string
	Reviewers []string
}

// ParseFrontmatter extracts YAML frontmatter from markdown content.
// Returns the parsed frontmatter and the remaining content without frontmatter.
func ParseFrontmatter(content []byte) (Frontmatter, []byte) {
	fm := Frontmatter{}
	text := string(content)

	// Check for frontmatter delimiter at the start
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return fm, content
	}

	// Find the closing delimiter
	var endIndex int
	if strings.HasPrefix(text, "---\r\n") {
		endIndex = strings.Index(text[5:], "\n---")
		if endIndex != -1 {
			endIndex += 5
		}
	} else {
		endIndex = strings.Index(text[4:], "\n---")
		if endIndex != -1 {
			endIndex += 4
		}
	}

	if endIndex == -1 {
		return fm, content
	}

	// Extract frontmatter block
	var fmBlock string
	if strings.HasPrefix(text, "---\r\n") {
		fmBlock = text[5:endIndex]
	} else {
		fmBlock = text[4:endIndex]
	}

	// Parse key-value pairs
	lines := strings.Split(fmBlock, "\n")
	var currentListKey string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			currentListKey = ""
			continue
		}

		// Handle YAML list items (e.g., "  - value")
		if strings.HasPrefix(trimmed, "- ") && currentListKey != "" {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			item = strings.Trim(item, "\"'")
			switch currentListKey {
			case "tags":
				fm.Tags = append(fm.Tags, item)
			case "reviewers":
				fm.Reviewers = append(fm.Reviewers, item)
			}
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			currentListKey = ""
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		lowerKey := strings.ToLower(key)

		// Remove surrounding quotes if present
		value = strings.Trim(value, "\"'")

		// Check for inline list syntax: [item1, item2]
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			items := parseList(value)
			switch lowerKey {
			case "tags":
				fm.Tags = items
			case "reviewers":
				fm.Reviewers = items
			}
			currentListKey = ""
			continue
		}

		// If value is empty, the next lines may be YAML list items
		if value == "" {
			if lowerKey == "tags" || lowerKey == "reviewers" {
				currentListKey = lowerKey
			}
			continue
		}

		currentListKey = ""

		// Comma-separated values for list fields
		switch lowerKey {
		case "title":
			fm.Title = value
		case "author":
			fm.Author = value
		case "status":
			fm.Status = value
		case "date":
			fm.Date = value
		case "tags":
			fm.Tags = parseList(value)
		case "category":
			fm.Category = value
		case "version":
			fm.Version = value
		case "reviewers":
			fm.Reviewers = parseList(value)
		}
	}

	// Find the end of the closing delimiter line
	remaining := text[endIndex+4:] // Skip "\n---"
	if strings.HasPrefix(remaining, "\r\n") {
		remaining = remaining[2:]
	} else if strings.HasPrefix(remaining, "\n") {
		remaining = remaining[1:]
	}

	return fm, []byte(remaining)
}

// parseList splits a comma-separated or YAML inline list value into trimmed items.
// Supports both "a, b, c" and "[a, b, c]" formats.
func parseList(value string) []string {
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")

	var items []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, "\"'")
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

// Renderer handles markdown to HTML conversion.
type Renderer struct {
	md goldmark.Markdown
}

// New creates a new Renderer with all necessary extensions enabled.
func New() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (tables, autolinks, strikethrough, etc.)
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)

	return &Renderer{md: md}
}

// Render converts markdown content to HTML.
func (r *Renderer) Render(content []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(content, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderWithLinks converts markdown to HTML and rewrites internal .md links.
// The currentDir parameter is the directory of the current file being rendered
// (relative to the base), used for resolving relative links.
func (r *Renderer) RenderWithLinks(content []byte, currentDir string) ([]byte, error) {
	htmlOut, err := r.Render(content)
	if err != nil {
		return nil, err
	}

	htmlOut = RewriteLinks(htmlOut, currentDir)
	htmlOut = TransformAdmonitions(htmlOut)

	return htmlOut, nil
}

// admonitionTypes maps GitHub-style alert markers to their display titles.
var admonitionTypes = map[string]string{
	"NOTE":      "Note",
	"TIP":       "Tip",
	"IMPORTANT": "Important",
	"WARNING":   "Warning",
	"CAUTION":   "Caution",
	"DANGER":    "Danger",
}

// admonitionPattern matches blockquotes containing GitHub-style alert markers.
// It captures the alert type from patterns like: <blockquote>\n<p>[!NOTE]<br> or <blockquote>\n<p>[!NOTE]</p>
var admonitionPattern = regexp.MustCompile(
	`(?s)<blockquote>\s*<p>\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION|DANGER)\](<br>\n?|\s*</p>)`,
)

// TransformAdmonitions converts GitHub-style alert blockquotes into styled admonition blocks.
// Input like `> [!NOTE]\n> text` (rendered by goldmark as a blockquote) becomes a
// blockquote with admonition classes and a title paragraph.
func TransformAdmonitions(htmlContent []byte) []byte {
	return admonitionPattern.ReplaceAllFunc(htmlContent, func(match []byte) []byte {
		sub := admonitionPattern.FindSubmatch(match)
		if len(sub) < 3 {
			return match
		}

		alertType := string(sub[1])
		title, ok := admonitionTypes[alertType]
		if !ok {
			return match
		}

		suffix := string(sub[2])
		class := strings.ToLower(alertType)

		// If the marker was followed by <br>, the remaining text continues in the same <p>.
		// Replace the marker+br with a title paragraph and reopen <p> for the rest.
		if strings.HasPrefix(suffix, "<br>") {
			return []byte(fmt.Sprintf(
				`<blockquote class="admonition admonition-%s">`+"\n"+
					`<p class="admonition-title">%s</p>`+"\n"+
					`<p>`, class, title))
		}

		// The marker was the only content in its <p> tag — replace it with a title.
		return []byte(fmt.Sprintf(
			`<blockquote class="admonition admonition-%s">`+"\n"+
				`<p class="admonition-title">%s</p>`, class, title))
	})
}

// linkPattern matches markdown-style links in HTML: href="something.md" or href="./path/to/file.md"
var linkPattern = regexp.MustCompile(`href="([^"]*\.(?:md|MD))"`)

// RewriteLinks transforms .md links to server routes.
// External links (http://, https://) are preserved.
// currentDir is the directory context for resolving relative paths.
func RewriteLinks(htmlContent []byte, currentDir string) []byte {
	return linkPattern.ReplaceAllFunc(htmlContent, func(match []byte) []byte {
		// Extract the link path
		matches := linkPattern.FindSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		linkPath := string(matches[1])

		// Skip external links
		if strings.HasPrefix(linkPath, "http://") || strings.HasPrefix(linkPath, "https://") {
			return match
		}

		// Resolve the path
		resolvedPath := resolveLink(linkPath, currentDir)

		// Remove .md extension and create server route
		resolvedPath = strings.TrimSuffix(resolvedPath, ".md")
		resolvedPath = strings.TrimSuffix(resolvedPath, ".MD")

		// Ensure it starts with /
		if !strings.HasPrefix(resolvedPath, "/") {
			resolvedPath = "/" + resolvedPath
		}

		return []byte(`href="` + resolvedPath + `"`)
	})
}

// resolveLink resolves a relative or absolute link path.
func resolveLink(linkPath, currentDir string) string {
	// Handle absolute paths (starting with /)
	if strings.HasPrefix(linkPath, "/") {
		return linkPath[1:] // Remove leading slash for processing
	}

	// Handle relative paths
	if currentDir == "" || currentDir == "." {
		return linkPath
	}

	// Join the current directory with the relative link
	joined := path.Join(currentDir, linkPath)

	// Clean the path to resolve .. and .
	return path.Clean(joined)
}
