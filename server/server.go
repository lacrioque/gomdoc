// Package server provides the HTTP server for serving markdown files.
package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gomdoc/mcpserver"
	"gomdoc/renderer"
	"gomdoc/scanner"
	"gomdoc/search"
	"gomdoc/templates"
)

// Server is the markdown HTTP server.
type Server struct {
	baseDir  string
	port     int
	title    string
	authUser string
	authPass string
	version  string
	renderer *renderer.Renderer
	index    *search.Index
}

// New creates a new Server instance.
func New(baseDir string, port int, title, authUser, authPass, version string) *Server {
	return &Server{
		baseDir:  baseDir,
		port:     port,
		title:    title,
		authUser: authUser,
		authPass: authPass,
		version:  version,
		renderer: renderer.New(),
		index:    search.NewIndex(),
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	// Build search index at startup
	if err := s.index.Build(s.baseDir); err != nil {
		log.Printf("Warning: failed to build search index: %v", err)
	} else {
		log.Printf("Search index built successfully")
	}

	// Set up MCP server on the same port
	mcpSrv := mcpserver.New(s.baseDir, s.version)
	if err := mcpSrv.BuildIndex(); err != nil {
		log.Printf("Warning: failed to build MCP index: %v", err)
	}

	mux := http.NewServeMux()

	mux.Handle("/mcp/", mcpSrv.SSEHandler())
	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/static/", s.handleStatic)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting gomdoc on http://localhost%s", addr)
	log.Printf("MCP server available at http://localhost%s/mcp/", addr)
	log.Printf("Serving files from: %s", s.baseDir)

	// Wrap with basic auth middleware if credentials are configured
	var handler http.Handler = mux
	if s.authUser != "" {
		log.Printf("Basic authentication enabled")
		handler = s.basicAuthMiddleware(mux)
	}

	return http.ListenAndServe(addr, handler)
}

// basicAuthMiddleware wraps a handler with HTTP Basic Authentication.
func (s *Server) basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != s.authUser || pass != s.authPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="gomdoc"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleRequest routes requests to either the index or a markdown file.
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Index page
	if path == "/" {
		s.handleIndex(w, r)
		return
	}

	// Markdown file
	s.handleMarkdown(w, r)
}

// handleIndex renders the file tree index page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	entries, err := scanner.ScanDirectory(s.baseDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error scanning directory: %v", err), http.StatusInternalServerError)
		return
	}

	tree := scanner.BuildTree(entries)
	treeHTML := scanner.RenderTree(tree)

	data := templates.IndexData{
		Title:     "Index",
		SiteTitle: s.title,
		TreeHTML:  template.HTML(treeHTML),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.RenderIndex(w, data); err != nil {
		log.Printf("Error rendering index: %v", err)
	}
}

// handleMarkdown renders a markdown file as HTML.
func (s *Server) handleMarkdown(w http.ResponseWriter, r *http.Request) {
	// Convert URL path to file path
	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	filePath := filepath.Join(s.baseDir, urlPath+".md")

	// Try lowercase .md first, then uppercase .MD
	content, err := os.ReadFile(filePath)
	if err != nil {
		filePath = filepath.Join(s.baseDir, urlPath+".MD")
		content, err = os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
	}

	// Parse frontmatter before rendering
	frontmatter, content := renderer.ParseFrontmatter(content)

	// Get the directory of the current file for link resolution
	currentDir := filepath.Dir(urlPath)
	if currentDir == "." {
		currentDir = ""
	}

	// Render markdown to HTML
	html, err := s.renderer.RenderWithLinks(content, currentDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering markdown: %v", err), http.StatusInternalServerError)
		return
	}

	// Use frontmatter title if available, otherwise use filename
	title := frontmatter.Title
	if title == "" {
		title = filepath.Base(urlPath)
	}

	data := templates.PageData{
		Title:     title,
		SiteTitle: s.title,
		Author:    frontmatter.Author,
		Content:   template.HTML(html),
		Path:      r.URL.Path,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.RenderPage(w, data); err != nil {
		log.Printf("Error rendering page: %v", err)
	}
}

// handleStatic serves embedded static files.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")

	if path == "style.css" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(styleCSS))
		return
	}

	http.NotFound(w, r)
}

// handleSearch responds with JSON search results for a query parameter.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	results := s.index.SearchKeywords(query, 20)
	if results == nil {
		results = []search.Result{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// styleCSS is the embedded CSS for styling the pages.
const styleCSS = `/* Theme custom properties */
:root {
    --color-bg: #fafafa;
    --color-text: #333;
    --color-text-muted: #555;
    --color-text-faint: #888;
    --color-text-quote: #666;
    --color-heading: #222;
    --color-link: #0066cc;
    --color-border: #e0e0e0;
    --color-border-light: #eee;
    --color-border-input: #ccc;
    --color-surface: #fff;
    --color-surface-alt: #f5f5f5;
    --color-surface-hover: #f0f0f0;
    --color-surface-code: #f5f5f5;
    --color-pre-bg: #282c34;
    --color-pre-text: #abb2bf;
    --color-blockquote-bg: #f9f9f9;
    --color-blockquote-border: #ddd;
    --color-table-border: #ddd;
    --color-table-header-bg: #f5f5f5;
    --color-table-stripe: #fafafa;
    --color-search-hover: #f5f8ff;
    --color-search-result-border: #f0f0f0;
    --color-shadow: rgba(0,0,0,0.1);
    --color-shadow-strong: rgba(0,0,0,0.15);
    --color-focus-ring: rgba(0,102,204,0.2);
    --color-mermaid-bg: #fff;
}

[data-theme="dark"] {
    --color-bg: #1a1a2e;
    --color-text: #e0e0e0;
    --color-text-muted: #b0b0b0;
    --color-text-faint: #808080;
    --color-text-quote: #aaa;
    --color-heading: #f0f0f0;
    --color-link: #6cb4ee;
    --color-border: #333;
    --color-border-light: #2a2a3e;
    --color-border-input: #444;
    --color-surface: #16213e;
    --color-surface-alt: #1a1a2e;
    --color-surface-hover: #1f2b47;
    --color-surface-code: #1e2a3a;
    --color-pre-bg: #0f1923;
    --color-pre-text: #abb2bf;
    --color-blockquote-bg: #1e2a3a;
    --color-blockquote-border: #444;
    --color-table-border: #333;
    --color-table-header-bg: #1e2a3a;
    --color-table-stripe: #1a2236;
    --color-search-hover: #1e2a3a;
    --color-search-result-border: #2a2a3e;
    --color-shadow: rgba(0,0,0,0.3);
    --color-shadow-strong: rgba(0,0,0,0.4);
    --color-focus-ring: rgba(108,180,238,0.3);
    --color-mermaid-bg: #16213e;
}

@media (prefers-color-scheme: dark) {
    :root:not([data-theme="light"]) {
        --color-bg: #1a1a2e;
        --color-text: #e0e0e0;
        --color-text-muted: #b0b0b0;
        --color-text-faint: #808080;
        --color-text-quote: #aaa;
        --color-heading: #f0f0f0;
        --color-link: #6cb4ee;
        --color-border: #333;
        --color-border-light: #2a2a3e;
        --color-border-input: #444;
        --color-surface: #16213e;
        --color-surface-alt: #1a1a2e;
        --color-surface-hover: #1f2b47;
        --color-surface-code: #1e2a3a;
        --color-pre-bg: #0f1923;
        --color-pre-text: #abb2bf;
        --color-blockquote-bg: #1e2a3a;
        --color-blockquote-border: #444;
        --color-table-border: #333;
        --color-table-header-bg: #1e2a3a;
        --color-table-stripe: #1a2236;
        --color-search-hover: #1e2a3a;
        --color-search-result-border: #2a2a3e;
        --color-shadow: rgba(0,0,0,0.3);
        --color-shadow-strong: rgba(0,0,0,0.4);
        --color-focus-ring: rgba(108,180,238,0.3);
        --color-mermaid-bg: #16213e;
    }
}

/* Base styles */
* {
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    line-height: 1.6;
    color: var(--color-text);
    max-width: 900px;
    margin: 0 auto;
    padding: 20px;
    background-color: var(--color-bg);
}

/* Navigation */
.nav-buttons {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 0;
    margin-bottom: 20px;
    border-bottom: 1px solid var(--color-border);
}

.nav-btn {
    padding: 8px 16px;
    border: 1px solid var(--color-border-input);
    background: var(--color-surface);
    color: var(--color-text);
    cursor: pointer;
    border-radius: 4px;
    font-size: 14px;
    transition: background-color 0.2s;
}

.nav-btn:hover {
    background-color: var(--color-surface-hover);
}

.nav-title {
    font-weight: bold;
    font-size: 18px;
    color: var(--color-text-muted);
}

.current-path {
    color: var(--color-text-faint);
    font-size: 14px;
    margin-left: auto;
}

/* Theme toggle */
.theme-toggle {
    padding: 6px 10px;
    border: 1px solid var(--color-border-input);
    background: var(--color-surface);
    color: var(--color-text);
    cursor: pointer;
    border-radius: 4px;
    font-size: 16px;
    line-height: 1;
    transition: background-color 0.2s;
}

.theme-toggle:hover {
    background-color: var(--color-surface-hover);
}

/* Content area */
.content {
    background: var(--color-surface);
    padding: 30px;
    border-radius: 8px;
    box-shadow: 0 1px 3px var(--color-shadow);
}

/* File tree */
.file-tree {
    list-style: none;
    padding-left: 0;
}

.file-tree ul {
    list-style: none;
    padding-left: 20px;
    margin: 5px 0;
}

.file-tree li {
    padding: 3px 0;
}

.file-tree .folder-details {
    margin: 0;
}

.file-tree .folder-details > ul {
    margin-top: 2px;
}

.file-tree .folder {
    font-weight: bold;
    color: var(--color-text-muted);
    cursor: pointer;
    list-style: none;
}

.file-tree .folder::-webkit-details-marker {
    display: none;
}

.file-tree .folder::before {
    content: "📁 ";
}

.file-tree details[open] > .folder::before {
    content: "📂 ";
}

.file-tree .file::before {
    content: "📄 ";
}

.file-tree a {
    color: var(--color-link);
    text-decoration: none;
}

.file-tree a:hover {
    text-decoration: underline;
}

/* Markdown content styles */
.content h1, .content h2, .content h3, .content h4, .content h5, .content h6 {
    margin-top: 1.5em;
    margin-bottom: 0.5em;
    color: var(--color-heading);
}

.content h1 { font-size: 2em; border-bottom: 2px solid var(--color-border-light); padding-bottom: 0.3em; }
.content h2 { font-size: 1.5em; border-bottom: 1px solid var(--color-border-light); padding-bottom: 0.3em; }
.content h3 { font-size: 1.25em; }

.content p {
    margin: 1em 0;
}

.content a {
    color: var(--color-link);
}

.content code {
    background-color: var(--color-surface-code);
    padding: 2px 6px;
    border-radius: 3px;
    font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
    font-size: 0.9em;
}

.content pre {
    background-color: var(--color-pre-bg);
    color: var(--color-pre-text);
    padding: 16px;
    border-radius: 6px;
    overflow-x: auto;
}

.content pre code {
    background: none;
    padding: 0;
    color: inherit;
}

.content blockquote {
    border-left: 4px solid var(--color-blockquote-border);
    margin: 1em 0;
    padding: 0.5em 1em;
    background-color: var(--color-blockquote-bg);
    color: var(--color-text-quote);
}

/* Admonition/callout blocks (GitHub-style alerts) */
.content .admonition {
    border-radius: 6px;
    padding: 12px 16px;
    color: #333;
}

.content .admonition .admonition-title {
    font-weight: 700;
    margin: 0 0 0.4em 0;
}

.content .admonition-note {
    border-left-color: #0969da;
    background-color: #ddf4ff;
}

.content .admonition-note .admonition-title { color: #0969da; }

.content .admonition-tip {
    border-left-color: #1a7f37;
    background-color: #dafbe1;
}

.content .admonition-tip .admonition-title { color: #1a7f37; }

.content .admonition-important {
    border-left-color: #8250df;
    background-color: #fbefff;
}

.content .admonition-important .admonition-title { color: #8250df; }

.content .admonition-warning {
    border-left-color: #9a6700;
    background-color: #fff8c5;
}

.content .admonition-warning .admonition-title { color: #9a6700; }

.content .admonition-caution {
    border-left-color: #cf222e;
    background-color: #ffebe9;
}

.content .admonition-caution .admonition-title { color: #cf222e; }

.content .admonition-danger {
    border-left-color: #cf222e;
    background-color: #ffebe9;
}

.content .admonition-danger .admonition-title { color: #cf222e; }

.content table {
    border-collapse: collapse;
    width: 100%;
    margin: 1em 0;
}

.content th, .content td {
    border: 1px solid var(--color-table-border);
    padding: 8px 12px;
    text-align: left;
}

.content th {
    background-color: var(--color-table-header-bg);
    font-weight: bold;
}

.content tr:nth-child(even) {
    background-color: var(--color-table-stripe);
}

.content img {
    max-width: 100%;
    height: auto;
}

.content ul, .content ol {
    margin: 1em 0;
    padding-left: 2em;
}

.content li {
    margin: 0.5em 0;
}

/* Mermaid diagrams */
.mermaid {
    background: var(--color-mermaid-bg);
    padding: 20px;
    border-radius: 4px;
    text-align: center;
}

/* Search */
.search-box {
    position: relative;
    flex: 1;
    max-width: 300px;
}

.search-box input {
    width: 100%;
    padding: 6px 12px;
    border: 1px solid var(--color-border-input);
    border-radius: 4px;
    font-size: 14px;
    outline: none;
    background: var(--color-surface);
    color: var(--color-text);
}

.search-box input:focus {
    border-color: var(--color-link);
    box-shadow: 0 0 0 2px var(--color-focus-ring);
}

.search-results {
    display: none;
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background: var(--color-surface);
    border: 1px solid var(--color-border-input);
    border-radius: 4px;
    margin-top: 4px;
    max-height: 400px;
    overflow-y: auto;
    box-shadow: 0 4px 12px var(--color-shadow-strong);
    z-index: 100;
}

.search-result {
    display: block;
    padding: 10px 12px;
    text-decoration: none;
    border-bottom: 1px solid var(--color-search-result-border);
    color: inherit;
}

.search-result:last-child {
    border-bottom: none;
}

.search-result:hover {
    background-color: var(--color-search-hover);
}

.search-result-title {
    font-weight: 600;
    color: var(--color-link);
    font-size: 14px;
}

.search-result-snippet {
    font-size: 12px;
    color: var(--color-text-quote);
    margin-top: 2px;
    line-height: 1.4;
}

.search-no-results {
    padding: 12px;
    color: var(--color-text-faint);
    font-size: 14px;
    text-align: center;
}

/* Index page */
.index-content h1 {
    margin-top: 0;
}

/* Footer */
.site-footer {
    margin-top: 40px;
    padding: 20px 0;
    border-top: 1px solid var(--color-border);
    text-align: center;
    font-size: 14px;
    color: var(--color-text-faint);
}

.site-footer a {
    color: var(--color-link);
    text-decoration: none;
}

.site-footer a:hover {
    text-decoration: underline;
}

/* Responsive: Tablet (768px) */
@media (max-width: 768px) {
    body {
        max-width: 100%;
        padding: 16px;
    }

    .nav-buttons {
        flex-wrap: wrap;
        gap: 8px;
    }

    .search-box {
        order: 10;
        flex-basis: 100%;
        max-width: 100%;
    }

    .current-path {
        margin-left: 0;
        flex-basis: 100%;
        order: 11;
    }

    .content {
        padding: 20px;
    }

    .content h1 { font-size: 1.6em; }
    .content h2 { font-size: 1.3em; }
    .content h3 { font-size: 1.1em; }
}

/* Responsive: Mobile (480px) */
@media (max-width: 480px) {
    body {
        padding: 10px;
        font-size: 15px;
    }

    .nav-buttons {
        flex-direction: column;
        align-items: stretch;
        gap: 6px;
    }

    .nav-btn {
        padding: 10px 14px;
        font-size: 15px;
        width: 100%;
        text-align: center;
    }

    .nav-title {
        font-size: 16px;
        text-align: center;
    }

    .search-box {
        order: unset;
        max-width: 100%;
    }

    .search-box input {
        padding: 10px 12px;
        font-size: 15px;
    }

    .current-path {
        font-size: 13px;
        text-align: center;
        order: unset;
    }

    .content {
        padding: 14px;
        border-radius: 4px;
    }

    .content h1 { font-size: 1.4em; }
    .content h2 { font-size: 1.15em; }

    .content pre {
        padding: 12px;
        font-size: 0.85em;
    }

    .content table {
        display: block;
        overflow-x: auto;
    }

    .content th, .content td {
        padding: 6px 8px;
        font-size: 14px;
    }

    .file-tree ul {
        padding-left: 14px;
    }

    .site-footer {
        font-size: 13px;
    }

    .search-results {
        max-height: 60vh;
    }
}

/* Print header (hidden on screen) */
.print-header {
    display: none;
}

/* Print styles */
@media print {

    html, body {
        background: white;
        color: #333;
        max-width: 100%;
        margin: 0;
        }

    @page {
        size: A4;
        margin: 0;
        padding: 12mm 16mm 24mm 12mm;
    }

    .nav-buttons, .search-box {
        display: none !important;
    }

    .site-footer {
        display: none !important;
    }

    .print-header {
        display: block !important;
        margin-bottom: 20px;
        padding-bottom: 10px;
        border-bottom: 2px solid #333;
    }

    .print-title {
        margin: 0;
        font-size: 24pt;
    }

    .print-author {
        margin: 5px 0 0 0;
        font-size: 12pt;
        color: #555;
    }

    .content {
        box-shadow: none;
        padding: 0;
        background: white;
    }

    .content pre {
        background-color: #f5f5f5 !important;
        color: #333 !important;
        border: 1px solid #ddd;
    }

    a {
        color: #000 !important;
        text-decoration: underline;
    }

    /* Prevent page breaks inside elements and break before h1*/

    .content h1:not(:first-of-type) {
        page-break-before: always;
    }

    .content h1, .content h2, .content h3,
    .content h4, .content h5, .content h6 {
        page-break-after: avoid;
    }

    .content pre, .content blockquote, .content table {
        page-break-inside: avoid;
    }

    .content p {
        orphans: 3;
        widows: 3;
    }
}`
