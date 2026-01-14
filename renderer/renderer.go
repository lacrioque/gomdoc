// Package renderer provides markdown to HTML conversion with link rewriting.
package renderer

import (
	"bytes"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

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
	html, err := r.Render(content)
	if err != nil {
		return nil, err
	}

	return RewriteLinks(html, currentDir), nil
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
