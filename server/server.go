// Package server provides the HTTP server for serving markdown files.
package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gomdoc/renderer"
	"gomdoc/scanner"
	"gomdoc/templates"
)

// Server is the markdown HTTP server.
type Server struct {
	baseDir  string
	port     int
	title    string
	authUser string
	authPass string
	renderer *renderer.Renderer
}

// New creates a new Server instance.
func New(baseDir string, port int, title, authUser, authPass string) *Server {
	return &Server{
		baseDir:  baseDir,
		port:     port,
		title:    title,
		authUser: authUser,
		authPass: authPass,
		renderer: renderer.New(),
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/static/", s.handleStatic)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting gomdoc on http://localhost%s", addr)
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

// styleCSS is the embedded CSS for styling the pages.
const styleCSS = `/* Base styles */
* {
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    line-height: 1.6;
    color: #333;
    max-width: 900px;
    margin: 0 auto;
    padding: 20px;
    background-color: #fafafa;
}

/* Navigation */
.nav-buttons {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 0;
    margin-bottom: 20px;
    border-bottom: 1px solid #e0e0e0;
}

.nav-btn {
    padding: 8px 16px;
    border: 1px solid #ccc;
    background: #fff;
    cursor: pointer;
    border-radius: 4px;
    font-size: 14px;
    transition: background-color 0.2s;
}

.nav-btn:hover {
    background-color: #f0f0f0;
}

.nav-title {
    font-weight: bold;
    font-size: 18px;
    color: #555;
}

.current-path {
    color: #888;
    font-size: 14px;
    margin-left: auto;
}

/* Content area */
.content {
    background: #fff;
    padding: 30px;
    border-radius: 8px;
    box-shadow: 0 1px 3px rgba(0,0,0,0.1);
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

.file-tree .folder {
    font-weight: bold;
    color: #555;
}

.file-tree .folder::before {
    content: "📁 ";
}

.file-tree .file::before {
    content: "📄 ";
}

.file-tree a {
    color: #0066cc;
    text-decoration: none;
}

.file-tree a:hover {
    text-decoration: underline;
}

/* Markdown content styles */
.content h1, .content h2, .content h3, .content h4, .content h5, .content h6 {
    margin-top: 1.5em;
    margin-bottom: 0.5em;
    color: #222;
}

.content h1 { font-size: 2em; border-bottom: 2px solid #eee; padding-bottom: 0.3em; }
.content h2 { font-size: 1.5em; border-bottom: 1px solid #eee; padding-bottom: 0.3em; }
.content h3 { font-size: 1.25em; }

.content p {
    margin: 1em 0;
}

.content a {
    color: #0066cc;
}

.content code {
    background-color: #f5f5f5;
    padding: 2px 6px;
    border-radius: 3px;
    font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
    font-size: 0.9em;
}

.content pre {
    background-color: #282c34;
    color: #abb2bf;
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
    border-left: 4px solid #ddd;
    margin: 1em 0;
    padding: 0.5em 1em;
    background-color: #f9f9f9;
    color: #666;
}

.content table {
    border-collapse: collapse;
    width: 100%;
    margin: 1em 0;
}

.content th, .content td {
    border: 1px solid #ddd;
    padding: 8px 12px;
    text-align: left;
}

.content th {
    background-color: #f5f5f5;
    font-weight: bold;
}

.content tr:nth-child(even) {
    background-color: #fafafa;
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
    background: #fff;
    padding: 20px;
    border-radius: 4px;
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
    border-top: 1px solid #e0e0e0;
    text-align: center;
    font-size: 14px;
    color: #888;
}

.site-footer a {
    color: #0066cc;
    text-decoration: none;
}

.site-footer a:hover {
    text-decoration: underline;
}

/* Print header (hidden on screen) */
.print-header {
    display: none;
}

/* Print styles */
@media print {
    .nav-buttons {
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

    body {
        background: white;
        max-width: 100%;
        margin: 0;
        padding: 20px;
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

    /* Prevent page breaks inside elements */
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
